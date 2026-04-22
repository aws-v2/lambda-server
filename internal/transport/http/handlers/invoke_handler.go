package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/utils/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	github_nats "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type 	InvokeHandler struct {
	DB *database.DB
	Nats *event.NatsClient
	ResolveFunction func(identifier, userID string) (*database.Function, error)
	ResolveIdentifier func(c *gin.Context) string
	NatsPrefix string
}

func NewInvokeHandler(db *database.DB, nats *event.NatsClient, natsPrefix string) *InvokeHandler {
	h := &InvokeHandler{DB: db, Nats: nats, NatsPrefix: natsPrefix}

	h.ResolveIdentifier = func(c *gin.Context) string {
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

	h.ResolveFunction = func(identifier, userID string) (*database.Function, error) {
		if strings.HasPrefix(identifier, "arn:") {
			return db.GetFunctionByARN(identifier, userID)
		}
		return db.GetFunction(identifier, userID)
	}

	return h
}


func (h *InvokeHandler) Invoke(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.invoke"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := h.ResolveIdentifier(c)
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

	fn, err := h.ResolveFunction(identifier, userIDStr)
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

	go func() {
		msgData, _ := json.Marshal(msg)

		respData, err := h.Nats.Request(c.Request.Context(), h.NatsPrefix+".fargate.tasks.run", msgData, 5*time.Minute)

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