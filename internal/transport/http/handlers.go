package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/storage"
	"lambda/internal/logger"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	github_nats "github.com/nats-io/nats.go"
)

type LambdaHandlers struct {
	DB      *database.DB
	Nats    *event.NatsClient
	Storage *storage.Storage
}

func NewLambdaHandlers(db *database.DB, nats *event.NatsClient, storage *storage.Storage) *LambdaHandlers {
	return &LambdaHandlers{
		DB:      db,
		Nats:    nats,
		Storage: storage,
	}
}

func (h *LambdaHandlers) Invoke(c *gin.Context) {
	var req dto.InvokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if function exists in DB
	l := logger.ForContext(c.Request.Context())
	fn, err := h.DB.GetFunction(req.Name)
	if err != nil {
		l.Warn("Function lookup failed", zap.String("name", req.Name), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Function not found"})
		return
	}

	taskID := uuid.New().String()

	// Stringify the payload for the env var
	payloadData, _ := json.Marshal(req.Payload)

	// Prepare the Env map. Start with DB envs if any, then add dynamic PAYLOAD.
	envMap := make(map[string]string)
	if fn.Env != nil {
		for k, v := range fn.Env {
			envMap[k] = v
		}
	}
	envMap["PAYLOAD"] = string(payloadData)

	msg := dto.NatsMessage{
		TraceID: c.GetString("trace_id"),
		TaskID:  taskID,
		Type:    fn.Type,
		Image:   fn.Image,
		Execution: dto.ExecutionDetails{
			Kind:    fn.Execution.Kind,
			Path:    fn.Execution.Path,
			Command: fn.Execution.Command,
		},
		Resources: dto.ResourceDetails{
			CPU:    fn.Resources.CPU,
			Memory: fn.Resources.Memory,
		},
		Env: envMap,
	}

	// setup SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Create a channel to communicate events to the main loop
	eventChan := make(chan string)

	// Create a done channel to signal the NATS callback to stop
	doneChan := make(chan struct{})
	defer close(doneChan)
	// defer close(eventChan) // We don't strictly need to close this if we rely on GC, but c.Stream might need it.
	// If c.Stream reads until close, we need to close it.
	// But c.Stream API takes a callback that returns bool. We return false on "DONE".
	// So we don't *need* to close eventChan to stop c.Stream. We stop c.Stream by returning false.
	// So let's NOT close eventChan manually to avoid the panic.

	// Subscribe to status updates for this task
	sub, err := h.Nats.Subscribe("task.status.>", func(m *github_nats.Msg) {
		// Check if we are done before sending
		select {
		case <-doneChan:
			return
		default:
		}

		// Parse message to see if it belongs to our taskID
		var statusMsg map[string]interface{}
		if err := json.Unmarshal(m.Data, &statusMsg); err == nil {
			if tid, ok := statusMsg["task_id"].(string); ok && tid == taskID {
				// Non-blocking send or select with done
				select {
				case eventChan <- string(m.Data):
				case <-doneChan:
					return
				}
			}
		}
	})
	if err != nil {
		l.Error("Failed to subscribe to statuses", zap.String("taskID", taskID), zap.Error(err))
		c.SSEvent("error", "Failed to subscribe to status updates")
		return
	}
	defer sub.Unsubscribe()

	// Launch the actual Request in a goroutine
	go func() {
		msgData, _ := json.Marshal(msg)
		// Increase timeout to 5 minutes to allow for long pulls, trusting that SSE keeps client alive
		respData, err := h.Nats.Request(c.Request.Context(), "tasks.run", msgData, 5*time.Minute)

		select {
		case <-doneChan:
			return
		default:
		}

		if err != nil {
			l.Error("NATS request failed", zap.String("taskID", taskID), zap.Error(err))
			select {
			case eventChan <- fmt.Sprintf(`{"status":"error", "message":"%v"}`, err):
			case <-doneChan:
			}
			return
		}
		// Final response
		select {
		case eventChan <- string(respData):
			select {
			case eventChan <- "DONE":
			case <-doneChan:
			}
		case <-doneChan:
		}
	}()

	// Stream events to client
	c.Stream(func(w io.Writer) bool {
		if msg, ok := <-eventChan; ok {
			if msg == "DONE" {
				return false
			}
			c.SSEvent("status", msg)
			return true
		}
		return false
	})
}

func (h *LambdaHandlers) RegisterFunction(c *gin.Context) {
	// Bind multipart form fields to the nested DTO
	var req dto.RegisterFunctionRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Handle Artifact (File vs Image)
	var artifactPath string

	if req.Execution.Kind == "image" {
		// For pure docker images, we don't need a file.
		// We might want to ensure 'image' field is set, but we'll leave that to basic validation.
		log.Printf("Registering Docker Image function: %s (Image: %s)", req.Name, req.Image)
	} else {
		// For binary/script/zip, file is required
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "binary file is required for non-image functions"})
			return
		}

		openedFile, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
			return
		}
		defer openedFile.Close()


		// Check if it's a zip file
		ext := filepath.Ext(file.Filename)
		isZip := ext == ".zip" || file.Header.Get("Content-Type") == "application/zip" || file.Header.Get("Content-Type") == "application/x-zip-compressed"

		if isZip {
			artifactPath, err = h.Storage.SaveFunctionZip(req.Name, openedFile, file.Size)
			if err != nil {
				logger.Log.Error("Failed to save function zip", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save function zip"})
				return
			}
		} else {
			// Save binary to disk
			artifactPath, err = h.Storage.SaveFunctionBinary(req.Name, openedFile)
			if err != nil {
				logger.Log.Error("Failed to save function binary", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save function binary"})
				return
			}
		}
	}

	// Apply defaults
	if req.Type == "" {
		req.Type = "lambda"
	}
	if req.Execution.Kind == "" { // Fallback if not provided in form
		req.Execution.Kind = "binary"
	}
	if req.Resources.CPU <= 0 {
		req.Resources.CPU = 1 // Default to 1 CPU
	}
	if req.Resources.Memory <= 0 {
		req.Resources.Memory = 128 // Default to 128MB
	}

	// 2. Save metadata to Postgres
	// Map DTO to Database Entity
	err := h.DB.SaveFunction(database.Function{
		Name:  req.Name,
		Type:  req.Type,
		Image: req.Image,
		Execution: database.ExecutionDetails{
			Kind:    req.Execution.Kind,
			Path:    artifactPath, // Ignore user path, use actual storage path
			Command: req.Execution.Command,
		},
		Resources: database.ResourceDetails{
			CPU:    req.Resources.CPU,
			Memory: req.Resources.Memory,
		},
		Env:       req.Env,
		TimeoutMS: req.TimeoutMS,
	})
	if err != nil {
		l := logger.ForContext(c.Request.Context())
		l.Error("Failed to save function metadata", zap.String("name", req.Name), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save function metadata"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "function registered successfully",
		"name":          req.Name,
		"artifact_path": artifactPath,
	})
}
