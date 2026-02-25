package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
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
	name := c.Param("name")
	var payload map[string]interface{}

	if name == "" {
		// Fallback to old behavior if not in URL
		var req dto.InvokeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required in URL or JSON body"})
			return
		}
		name = req.Name
		payload = req.Payload
	} else {
		// Read payload if provided in body
		if c.Request.ContentLength > 0 {
			if err := c.ShouldBindJSON(&payload); err != nil {
				// We don't necessarily error out, maybe empty payload is fine
				logger.Log.Warn("Failed to bind invoke payload", zap.Error(err))
			}
		}
	}

	// Check if function exists in DB
	l := logger.ForContext(c.Request.Context())
	userID, _ := c.Get("user_id")
	userIDStr := ""
	if id, ok := userID.(string); ok {
		userIDStr = id
	}

	fn, err := h.DB.GetFunction(name, userIDStr)
	if err != nil {
		l.Warn("Function lookup failed", zap.String("name", name), zap.String("userID", userIDStr), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Function not found or access denied"})
		return
	}

	startTime := time.Now()
	metric := database.LambdaMetric{
		FunctionName: fn.Name,
		UserID:       userIDStr,
		Status:       "success",
	}

	defer func() {
		metric.DurationMS = int(time.Since(startTime).Milliseconds())
		h.DB.RecordMetric(metric)
	}()

	taskID := uuid.New().String()

	// Stringify the payload for the env var
	payloadData, _ := json.Marshal(payload)

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
		metric.Status = "error"
		metric.ErrorMessage = "Failed to subscribe to status updates"
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
			metric.Status = "error"
			metric.ErrorMessage = fmt.Sprintf("NATS request failed: %v", err)
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

			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(msg), &eventData); err == nil {
				if status, sOk := eventData["status"].(string); sOk && status == "error" {
					metric.Status = "error"
					if errMsg, mOk := eventData["message"].(string); mOk {
						metric.ErrorMessage = errMsg
					}
					c.SSEvent("status", gin.H{"error": eventData})
					return true
				}
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

	userID, _ := c.Get("user_id")
	uIDStr, _ := userID.(string)

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

	if req.Type == "" {
		req.Type = "lambda"
	}

	// Graceful JSON parsing for stringified fields (common in multipart forms)
	if len(req.Execution.Command) == 1 {
		cmdStr := req.Execution.Command[0]
		if strings.HasPrefix(cmdStr, "[") && strings.HasSuffix(cmdStr, "]") {
			var parsed []string
			if err := json.Unmarshal([]byte(cmdStr), &parsed); err == nil {
				req.Execution.Command = parsed
			}
		}
	}

	// Try to parse Env if it was sent as a single JSON string
	if len(req.Env) == 0 {
		rawEnv := c.PostForm("env")
		if rawEnv != "" && strings.HasPrefix(rawEnv, "{") && strings.HasSuffix(rawEnv, "}") {
			var parsed map[string]string
			if err := json.Unmarshal([]byte(rawEnv), &parsed); err == nil {
				req.Env = parsed
			}
		}
	}
	if req.Execution.Kind == "" { // Fallback if not provided in form
		req.Execution.Kind = "binary"
	}

	// Automatic image selection if not provided
	if req.Image == "" {
		switch req.Execution.Kind {
		case "python":
			req.Image = "python:3.10-slim"
		case "node", "nodejs":
			req.Image = "node:18-alpine"
		case "java":
			req.Image = "amazoncorretto:17"
		default:
			req.Image = "golang:1.22-alpine"
		}
	}
	if req.Resources.CPU <= 0 {
		req.Resources.CPU = 1 // Default to 1 CPU
	}
	if req.Resources.Memory <= 0 {
		req.Resources.Memory = 128 // Default to 128MB
	}

	// 2. Save metadata to Postgres
	// Map DTO to Database Entity
	// We force Command and Env to nil to leverage the fargate-server's internal robust execution logic
	err := h.DB.SaveFunction(database.Function{
		Name:   req.Name,
		UserID: uIDStr,
		Type:   req.Type,
		Image:  req.Image,
		Execution: database.ExecutionDetails{
			Kind:    req.Execution.Kind,
			Path:    artifactPath, // Ignore user path, use actual storage path
			Command: nil,          // Force null to use executor fallbacks
		},
		Resources: database.ResourceDetails{
			CPU:    req.Resources.CPU,
			Memory: req.Resources.Memory,
		},
		Env:         nil, // Force null to avoid empty object {} vs null issues
		TimeoutMS:   req.TimeoutMS,
		Description: req.Description,
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

func (h *LambdaHandlers) ListFunctions(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	functions, err := h.DB.ListFunctionsByUser(userIDStr)
	if err != nil {
		l := logger.ForContext(c.Request.Context())
		l.Error("Failed to list functions", zap.String("userID", userIDStr), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch functions"})
		return
	}

	c.JSON(http.StatusOK, functions)
}

func (h *LambdaHandlers) GetFunction(c *gin.Context) {
	name := c.Param("name")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	fn, err := h.DB.GetFunction(name, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	c.JSON(http.StatusOK, fn)
}

func (h *LambdaHandlers) GetCode(c *gin.Context) {
	name := c.Param("name")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	// Verify ownership
	_, err := h.DB.GetFunction(name, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	// Fetch code from storage
	// We'll try "handler" first, as it's our default binary/script name
	content, err := h.Storage.ReadFunctionFile(name, "handler")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "code artifact not found"})
		return
	}

	c.Data(http.StatusOK, "text/plain", content)
}

func (h *LambdaHandlers) UpdateCode(c *gin.Context) {
	name := c.Param("name")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	// Verify ownership
	_, err := h.DB.GetFunction(name, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	// Read new code from body
	content, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Save to storage
	err = h.Storage.WriteFunctionFile(name, "handler", content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update code artifact"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "code updated successfully"})
}

func (h *LambdaHandlers) GetMetrics(c *gin.Context) {
	name := c.Param("name")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	metrics, err := h.DB.GetMetrics(name, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch metrics"})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *LambdaHandlers) UpdateConfig(c *gin.Context) {
	name := c.Param("name")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	var req dto.UpdateFunctionConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.DB.UpdateFunctionConfig(name, userIDStr, req.Memory, req.Timeout, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update function configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "configuration updated successfully"})
}
