package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/auth"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/storage"
	"lambda/internal/utils/logger"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	github_nats "github.com/nats-io/nats.go"
)

type LambdaHandlers struct {
	DB       *database.DB
	Nats     *event.NatsClient
	Storage  *storage.Storage
	Resolver *auth.ApiKeyResolver
	Region   string
	Docs     *DocsHandler  // ← this line must be present
}
func NewLambdaHandlers(db *database.DB, nats *event.NatsClient, storage *storage.Storage, resolver *auth.ApiKeyResolver, region string, docsHandler *DocsHandler) *LambdaHandlers {
	return &LambdaHandlers{
		DB:       db,
		Nats:     nats,
		Storage:  storage,
		Resolver: resolver,
		Region:   region,
		Docs:     docsHandler,  // ← wire it here
	}
}
// resolveIdentifier extracts the function identifier (name or ARN) from the Gin context.
// Priority order:
//  1. _resolved_arn – set by ArnRouter after stripping the sub-path suffix
//  2. *arn wildcard param (raw, with leading slash stripped)
//  3. :name regular param (name-based routes)
func resolveIdentifier(c *gin.Context) string {
	if resolvedArn, exists := c.Get("_resolved_arn"); exists {
		if arn, ok := resolvedArn.(string); ok && arn != "" {
			return arn
		}
	}
	if arn := c.Param("arn"); arn != "" {
		return strings.TrimPrefix(arn, "/")
	}
	return c.Param("name")
}

// generateArn creates a Serwin Lambda ARN.
// Format: arn:serwin:lambda:<region>:<userID>:function:<name>
func (h *LambdaHandlers) generateArn(userID, name string) string {
	region := h.Region
	if region == "" {
		region = "eu-north-1"
	}
	return fmt.Sprintf("arn:serwin:lambda:%s:%s:function:%s", region, userID, name)
}

// resolveFunction looks up a function by name or ARN depending on the identifier.
func (h *LambdaHandlers) resolveFunction(identifier, userID string) (*database.Function, error) {
	if strings.HasPrefix(identifier, "arn:") {
		return h.DB.GetFunctionByARN(identifier, userID)
	}
	return h.DB.GetFunction(identifier, userID)
}

// ArnRouter handles ARN-based routing and delegates to the correct handler.
func (h *LambdaHandlers) ArnRouter(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.route"),
		zap.String(logger.F.Domain, "lambda"),
	)

	raw := strings.TrimPrefix(c.Param("arn"), "/")

	suffixes := []string{"/metrics", "/invoke", "/config", "/code", "/test"}
	suffix := ""
	arnStr := raw

	for _, s := range suffixes {
		if strings.HasSuffix(raw, s) {
			arnStr = raw[:len(raw)-len(s)]
			suffix = s
			break
		}
	}

	// Inject resolved ARN into context
	c.Set("_resolved_arn", arnStr)

	log.Info("routing ARN request",
		zap.String("function_arn", arnStr),
		zap.String("route_suffix", suffix),
	)

	switch suffix {
	case "/invoke", "/test":
		log.Debug("dispatching to invoke handler")
		h.Invoke(c)

	case "/metrics":
		log.Debug("dispatching to metrics handler")
		h.GetMetrics(c)

	case "/config":
		log.Debug("dispatching to config handler")
		h.UpdateConfig(c)

	case "/code":
		log.Debug("dispatching to code handler")
		h.UpdateCode(c)

	default:
		log.Debug("dispatching to get function handler")
		h.GetFunction(c)
	}
}




func (h *LambdaHandlers) Invoke(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.invoke"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := resolveIdentifier(c)
	var payload map[string]interface{}

	if identifier == "" {
		var req dto.InvokeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Warn("missing identifier in request",
				zap.String(logger.F.ErrorKind, "invalid_request"),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "name or arn is required in URL or JSON body"})
			return
		}
		identifier = req.Name
		payload = req.Payload
	} else {
		if c.Request.ContentLength > 0 {
			if err := c.ShouldBindJSON(&payload); err != nil {
				log.Warn("failed to bind invoke payload",
					zap.String(logger.F.ErrorKind, "decode_error"),
					zap.Error(err),
				)
			}
		}
	}

	userID, _ := c.Get("user_id")
	userIDStr := ""
	if id, ok := userID.(string); ok {
		userIDStr = id
	}

	log = log.With(
		zap.String("function_identifier", identifier),
		zap.String("user_id", userIDStr),
	)

	// Resolve function
	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
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

	log.Info("invocation started",
		zap.String("task_id", taskID),
		zap.String("function_name", fn.Name),
	)

	payloadData, _ := json.Marshal(payload)

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

	// SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	eventChan := make(chan string)
	doneChan := make(chan struct{})
	defer close(doneChan)

	// Subscribe
	sub, err := h.Nats.Subscribe("task.status.>", func(m *github_nats.Msg) {
		select {
		case <-doneChan:
			return
		default:
		}

		var statusMsg map[string]interface{}
		if err := json.Unmarshal(m.Data, &statusMsg); err == nil {
			if tid, ok := statusMsg["task_id"].(string); ok && tid == taskID {
				select {
				case eventChan <- string(m.Data):
				case <-doneChan:
					return
				}
			}
		}
	})
	if err != nil {
		log.Error("failed to subscribe to task status",
			zap.String(logger.F.ErrorKind, "nats_subscribe_error"),
			zap.String("task_id", taskID),
			zap.Error(err),
		)
		metric.Status = "error"
		metric.ErrorMessage = "Failed to subscribe to status updates"
		c.SSEvent("error", "Failed to subscribe to status updates")
		return
	}
	defer sub.Unsubscribe()

	// Execute task
	go func() {
		msgData, _ := json.Marshal(msg)

		respData, err := h.Nats.Request(c.Request.Context(), "tasks.run", msgData, 5*time.Minute)

		select {
		case <-doneChan:
			return
		default:
		}

		if err != nil {
			log.Error("task execution request failed",
				zap.String(logger.F.ErrorKind, "nats_request_error"),
				zap.String("task_id", taskID),
				zap.Error(err),
			)
			metric.Status = "error"
			metric.ErrorMessage = fmt.Sprintf("NATS request failed: %v", err)

			select {
			case eventChan <- fmt.Sprintf(`{"status":"error", "message":"%v"}`, err):
			case <-doneChan:
			}
			return
		}

		log.Info("task execution completed",
			zap.String("task_id", taskID),
		)

		select {
		case eventChan <- string(respData):
			select {
			case eventChan <- "DONE":
			case <-doneChan:
			}
		case <-doneChan:
		}
	}()

	// Stream back to client
	c.Stream(func(w io.Writer) bool {
		if msg, ok := <-eventChan; ok {
			if msg == "DONE" {
				log.Info("stream completed",
					zap.String("task_id", taskID),
				)
				return false
			}

			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(msg), &eventData); err == nil {
				if status, sOk := eventData["status"].(string); sOk && status == "error" {
					log.Warn("task reported error",
						zap.String(logger.F.ErrorKind, "execution_error"),
						zap.String("task_id", taskID),
					)

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
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.register"),
		zap.String(logger.F.Domain, "lambda"),
	)

	var req dto.RegisterFunctionRequest
	if err := c.ShouldBind(&req); err != nil {
		log.Warn("failed to bind register request",
			zap.String(logger.F.ErrorKind, "invalid_request"),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	uIDStr, _ := userID.(string)

	log = log.With(
		zap.String("function_name", req.Name),
		zap.String("user_id", uIDStr),
	)

	// 1. Handle Artifact
	var artifactPath string

	if req.Execution.Kind == "image" {
		log.Info("registering image-based function",
			zap.String("image", req.Image),
		)
	} else {
		file, err := c.FormFile("file")
		if err != nil {
			log.Warn("missing file for non-image function",
				zap.String(logger.F.ErrorKind, "invalid_request"),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "binary file is required for non-image functions"})
			return
		}

		openedFile, err := file.Open()
		if err != nil {
			log.Error("failed to open uploaded file",
				zap.String(logger.F.ErrorKind, "file_open_error"),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
			return
		}
		defer openedFile.Close()

		ext := filepath.Ext(file.Filename)
		isZip := ext == ".zip" ||
			file.Header.Get("Content-Type") == "application/zip" ||
			file.Header.Get("Content-Type") == "application/x-zip-compressed"

		if isZip {
			artifactPath, err = h.Storage.SaveFunctionZip(req.Name, openedFile, file.Size)
			if err != nil {
				log.Error("failed to save function zip",
					zap.String(logger.F.ErrorKind, "storage_error"),
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save function zip"})
				return
			}
			log.Info("function zip saved",
				zap.String("artifact_path", artifactPath),
			)
		} else {
			artifactPath, err = h.Storage.SaveFunctionBinary(req.Name, openedFile)
			if err != nil {
				log.Error("failed to save function binary",
					zap.String(logger.F.ErrorKind, "storage_error"),
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save function binary"})
				return
			}
			log.Info("function binary saved",
				zap.String("artifact_path", artifactPath),
			)
		}
	}

	if req.Type == "" {
		req.Type = "lambda"
	}

	if len(req.Execution.Command) == 1 {
		cmdStr := req.Execution.Command[0]
		if strings.HasPrefix(cmdStr, "[") && strings.HasSuffix(cmdStr, "]") {
			var parsed []string
			if err := json.Unmarshal([]byte(cmdStr), &parsed); err == nil {
				req.Execution.Command = parsed
			}
		}
	}

	if len(req.Env) == 0 {
		rawEnv := c.PostForm("env")
		if rawEnv != "" && strings.HasPrefix(rawEnv, "{") && strings.HasSuffix(rawEnv, "}") {
			var parsed map[string]string
			if err := json.Unmarshal([]byte(rawEnv), &parsed); err == nil {
				req.Env = parsed
			}
		}
	}

	if req.Execution.Kind == "" {
		req.Execution.Kind = "binary"
	}

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
		req.Resources.CPU = 1
	}
	if req.Resources.Memory <= 0 {
		req.Resources.Memory = 128
	}

	arn := h.generateArn(uIDStr, req.Name)

	err := h.DB.SaveFunction(database.Function{
		Name:   req.Name,
		ARN:    arn,
		UserID: uIDStr,
		Type:   req.Type,
		Image:  req.Image,
		Execution: database.ExecutionDetails{
			Kind:    req.Execution.Kind,
			Path:    artifactPath,
			Command: nil,
		},
		Resources: database.ResourceDetails{
			CPU:    req.Resources.CPU,
			Memory: req.Resources.Memory,
		},
		Env:         nil,
		TimeoutMS:   req.TimeoutMS,
		Description: req.Description,
	})
	if err != nil {
		log.Error("failed to save function metadata",
			zap.String(logger.F.ErrorKind, "db_write_error"),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save function metadata"})
		return
	}

	log.Info("function registered successfully",
		zap.String("function_arn", arn),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":       "function registered successfully",
		"name":          req.Name,
		"artifact_path": artifactPath,
	})
}

func (h *LambdaHandlers) ListFunctions(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.list"),
		zap.String(logger.F.Domain, "lambda"),
	)

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	functions, err := h.DB.ListFunctionsByUser(userIDStr)
	if err != nil {
		log.Error("failed to list functions",
			zap.String(logger.F.ErrorKind, "db_read_error"),
			zap.String("user_id", userIDStr),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch functions"})
		return
	}

	log.Info("functions retrieved successfully",
		zap.String("user_id", userIDStr),
		zap.Int("count", len(functions)),
	)

	c.JSON(http.StatusOK, functions)
}





















func (h *LambdaHandlers) GetFunction(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.get"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := resolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	log = log.With(
		zap.String("function_identifier", identifier),
		zap.String("user_id", userIDStr),
	)

	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	log.Info("function retrieved successfully",
		zap.String("function_name", fn.Name),
	)

	c.JSON(http.StatusOK, fn)
}

func (h *LambdaHandlers) GetCode(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.get_code"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := resolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	log = log.With(
		zap.String("function_identifier", identifier),
		zap.String("user_id", userIDStr),
	)

	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	content, err := h.Storage.ReadFunctionFile(fn.Name, "handler")
	if err != nil {
		log.Warn("code artifact not found",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.String("function_name", fn.Name),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "code artifact not found"})
		return
	}

	log.Info("function code retrieved",
		zap.String("function_name", fn.Name),
	)

	c.Data(http.StatusOK, "text/plain", content)
}

func (h *LambdaHandlers) UpdateCode(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.update_code"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := resolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	log = log.With(
		zap.String("function_identifier", identifier),
		zap.String("user_id", userIDStr),
	)

	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	content, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Warn("failed to read request body",
			zap.String(logger.F.ErrorKind, "read_error"),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	err = h.Storage.WriteFunctionFile(fn.Name, "handler", content)
	if err != nil {
		log.Error("failed to update code artifact",
			zap.String(logger.F.ErrorKind, "storage_error"),
			zap.String("function_name", fn.Name),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update code artifact"})
		return
	}

	log.Info("function code updated successfully",
		zap.String("function_name", fn.Name),
	)

	c.JSON(http.StatusOK, gin.H{"message": "code updated successfully"})
}

func (h *LambdaHandlers) GetMetrics(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.metrics"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := resolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	log = log.With(
		zap.String("function_identifier", identifier),
		zap.String("user_id", userIDStr),
	)

	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	metrics, err := h.DB.GetMetrics(fn.Name, userIDStr)
	if err != nil {
		log.Error("failed to fetch metrics",
			zap.String(logger.F.ErrorKind, "db_read_error"),
			zap.String("function_name", fn.Name),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch metrics"})
		return
	}

	log.Info("metrics retrieved successfully",
		zap.String("function_name", fn.Name),
	)

	c.JSON(http.StatusOK, metrics)
}

func (h *LambdaHandlers) UpdateConfig(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.update_config"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := resolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	log = log.With(
		zap.String("function_identifier", identifier),
		zap.String("user_id", userIDStr),
	)

	var req dto.UpdateFunctionConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("failed to bind config update request",
			zap.String(logger.F.ErrorKind, "invalid_request"),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	err = h.DB.UpdateFunctionConfig(fn.Name, userIDStr, req.Memory, req.Timeout, req.Description)
	if err != nil {
		log.Error("failed to update function configuration",
			zap.String(logger.F.ErrorKind, "db_write_error"),
			zap.String("function_name", fn.Name),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update function configuration"})
		return
	}

	log.Info("function configuration updated successfully",
		zap.String("function_name", fn.Name),
	)

	c.JSON(http.StatusOK, gin.H{"message": "configuration updated successfully"})
}
 