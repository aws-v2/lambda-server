package http

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/auth"
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
	// 1. ArnRouter sets this after splitting off the sub-path suffix.
	if resolvedArn, exists := c.Get("_resolved_arn"); exists {
		if arn, ok := resolvedArn.(string); ok && arn != "" {
			return arn
		}
	}
	// 2. Raw wildcard param (direct ARN routes without sub-path)
	if arn := c.Param("arn"); arn != "" {
		return strings.TrimPrefix(arn, "/")
	}
	// 3. Name-based param
	return c.Param("name")
}

// generateArn creates a Serwin Lambda ARN.
// Format: arn:serwin:lambda:<region>:<userID>:function:<name>
func (h *LambdaHandlers) generateArn(userID, name string) string {
	region := h.Region
	if region == "" {
		region = "eu-north-1" // Safe fallback
	}
	return fmt.Sprintf("arn:serwin:lambda:%s:%s:function:%s", region, userID, name)
}

// resolveFunction looks up a function by name or ARN depending on the identifier.
// If the identifier starts with "arn:" it uses the ARN-based lookup, otherwise by name.
func (h *LambdaHandlers) resolveFunction(identifier, userID string) (*database.Function, error) {
	if strings.HasPrefix(identifier, "arn:") {
		return h.DB.GetFunctionByARN(identifier, userID)
	}
	return h.DB.GetFunction(identifier, userID)
}

// ArnRouter is a catch-all handler for all ARN-based routes registered under
// /functions/arn/*arn. It splits the captured wildcard into the ARN itself and an
// optional sub-path suffix (e.g. "/metrics", "/config", "/invoke", "/code") and
// delegates to the appropriate handler.
//
// This is required because Gin does not allow registering multiple wildcard routes
// with the same prefix (e.g. /*arn AND /*arn/metrics would conflict).
func (h *LambdaHandlers) ArnRouter(c *gin.Context) {
	// The wildcard includes a leading slash: e.g. "/arn:serwin:lambda:...:function:foo/metrics"
	raw := strings.TrimPrefix(c.Param("arn"), "/")

	// Known suffixes — must be checked longest-first to avoid false matches.
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

	// Inject the clean ARN as the "arn" param so that resolveIdentifier picks it up.
	// We can't modify c.Params directly, so we store it in the context.
	c.Set("_resolved_arn", arnStr)

	switch suffix {
	case "/invoke", "/test":
		h.Invoke(c)
	case "/metrics":
		h.GetMetrics(c)
	case "/config":
		h.UpdateConfig(c)
	case "/code":
		h.UpdateCode(c)
	default:
		// No suffix → GET the function metadata
		h.GetFunction(c)
	}
}

func (h *LambdaHandlers) Invoke(c *gin.Context) {
	identifier := resolveIdentifier(c)
	var payload map[string]interface{}

	if identifier == "" {
		// Fallback to old behavior if not in URL
		var req dto.InvokeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name or arn is required in URL or JSON body"})
			return
		}
		identifier = req.Name
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

	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		l.Warn("Function lookup failed", zap.String("identifier", identifier), zap.String("userID", userIDStr), zap.Error(err))
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
	arn := h.generateArn(uIDStr, req.Name)
	err := h.DB.SaveFunction(database.Function{
		Name:   req.Name,
		ARN:    arn,
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
	identifier := resolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	c.JSON(http.StatusOK, fn)
}

func (h *LambdaHandlers) GetCode(c *gin.Context) {
	identifier := resolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	// Verify ownership & get function name (needed for storage path)
	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	// Fetch code from storage
	// We'll try "handler" first, as it's our default binary/script name
	content, err := h.Storage.ReadFunctionFile(fn.Name, "handler")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "code artifact not found"})
		return
	}

	c.Data(http.StatusOK, "text/plain", content)
}

func (h *LambdaHandlers) UpdateCode(c *gin.Context) {
	identifier := resolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	// Verify ownership & get function name (needed for storage path)
	fn, err := h.resolveFunction(identifier, userIDStr)
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
	err = h.Storage.WriteFunctionFile(fn.Name, "handler", content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update code artifact"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "code updated successfully"})
}

func (h *LambdaHandlers) GetMetrics(c *gin.Context) {
	identifier := resolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	// Resolve function to get canonical name (metrics are stored by name)
	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	metrics, err := h.DB.GetMetrics(fn.Name, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch metrics"})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *LambdaHandlers) UpdateConfig(c *gin.Context) {
	identifier := resolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	var req dto.UpdateFunctionConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve function to get canonical name (config update uses name)
	fn, err := h.resolveFunction(identifier, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	err = h.DB.UpdateFunctionConfig(fn.Name, userIDStr, req.Memory, req.Timeout, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update function configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "configuration updated successfully"})
}

// ── Documentation Handlers ────────────────────────────────────────────────────

// docsBasePath returns the path to the docs/lambda directory, resolved relative
// to the working directory of the running process (which is the project root when
// run via `go run` or the compiled binary).
func docsBasePath() string {
	return filepath.Join("docs", "lambda")
}

// GetManifest serves GET /api/v1/lambda/docs
// Returns the full docs table-of-contents from docs/lambda/manifest.json.
func (h *LambdaHandlers) GetManifest(c *gin.Context) {
	manifestPath := filepath.Join(docsBasePath(), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read docs manifest"})
		return
	}

	var manifest interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse docs manifest"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": manifest})
}

// docMetadata holds the parsed YAML front-matter fields from a .md file.
type docMetadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	LastUpdated string   `json:"lastUpdated"`
	Tags        []string `json:"tags"`
}

// parseDocFile reads a markdown file and splits it into front-matter metadata
// and body content. Front-matter is expected to be a YAML block delimited by
// triple-dashes (---) at the top of the file.
func parseDocFile(path string) (*docMetadata, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	content := string(data)
	meta := &docMetadata{}

	// Check for YAML front-matter block
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return meta, content, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var fmLines []string
	var bodyLines []string
	inFrontMatter := false
	fmClosed := false
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		if lineCount == 1 && line == "---" {
			inFrontMatter = true
			continue
		}
		if inFrontMatter && line == "---" {
			inFrontMatter = false
			fmClosed = true
			continue
		}
		if inFrontMatter {
			fmLines = append(fmLines, line)
		} else if fmClosed {
			bodyLines = append(bodyLines, line)
		}
	}

	// Simple key-value YAML parser (handles string scalars and string lists)
	parseSimpleYAML(fmLines, meta)

	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	return meta, body, nil
}

// parseSimpleYAML parses a flat YAML-like list of lines into docMetadata.
// Supports: plain scalar values and simple inline string lists (- item).
func parseSimpleYAML(lines []string, meta *docMetadata) {
	var currentKey string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// List item
		if strings.HasPrefix(trimmed, "- ") {
			value := strings.TrimPrefix(trimmed, "- ")
			value = strings.Trim(value, "\"'")
			if currentKey == "tags" {
				meta.Tags = append(meta.Tags, value)
			}
			continue
		}
		// Key: value
		if idx := strings.Index(trimmed, ":"); idx != -1 {
			key := strings.TrimSpace(trimmed[:idx])
			value := strings.TrimSpace(trimmed[idx+1:])
			value = strings.Trim(value, "\"'")
			currentKey = key
			switch key {
			case "title":
				meta.Title = value
			case "description":
				meta.Description = value
			case "icon":
				meta.Icon = value
			case "lastUpdated":
				meta.LastUpdated = value
			}
		}
	}
}

// GetDocBySlug serves GET /api/v1/lambda/docs/:slug
// Reads docs/lambda/<slug>.md, parses front-matter, and returns structured JSON.
func (h *LambdaHandlers) GetDocBySlug(c *gin.Context) {
	slug := c.Param("slug")

	// Sanitize: only allow alphanumeric and dashes to prevent path traversal
	for _, ch := range slug {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-') {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slug"})
			return
		}
	}

	docPath := filepath.Join(docsBasePath(), slug+".md")
	meta, content, err := parseDocFile(docPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("doc not found: %s", slug)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read doc"})
		return
	}

	// Fill in defaults for missing metadata
	if meta.LastUpdated == "" {
		meta.LastUpdated = time.Now().Format("2006-01-02")
	}
	if meta.Icon == "" {
		meta.Icon = "cpu"
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"metadata": meta,
			"content":  content,
		},
	})
}
