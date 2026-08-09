package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lambda/internal/domain"
	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/utils/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	github_nats "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type InvokeHandler struct {
	DB                *database.DB
	Nats              *event.NatsClient
	ResolveFunction   func(identifier, userID string) (*database.Function, error)
	ResolveIdentifier func(c *gin.Context) string
	NatsPrefix        string
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

type createPresignDownloadURLRequest struct {
	UserID        string `json:"user_id"`
	CorrelationID string `json:"correlation_id"`
	FileSha256    string `json:"sha256"`
	AssetID       string `json:"asset_id"`
	FileCount     int    `json:"file_count"`
}
type createPresignDownloadURLResponse struct {
	URL string `json:"url"`
}


type AssetConfigs struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type GameManifest struct {
	Parameters map[string]string `json:"parameters"`
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

	userID := c.GetString("userID")
	userIDStr := userID

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
		TraceID: uuid.New().String(),
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


		fmt.Printf("%T", msgData)

		uploadPresignUrl := fmt.Sprintf("%s.s3.task.create_presign_download_url", h.NatsPrefix)

		payload, _ := json.Marshal(createPresignDownloadURLRequest{
			UserID:        userID,
			CorrelationID: uuid.New().String(),
			FileSha256:    fn.Sha256,
			AssetID:       fn.ID,
			FileCount:     1,
		})

		s3UrlRespData, err := h.Nats.Request(c.Request.Context(), uploadPresignUrl, payload, 5*time.Minute)

		var respo createPresignDownloadURLResponse
		error := json.Unmarshal(s3UrlRespData, &respo)

		if error != nil {
			fmt.Printf("failed to Unmarshal payload: %w", err)

			return
			// return  fmt.Errorf("failed to Unmarshal payload: %w", err)
		}

		fmt.Printf("------99>*:%v", respo.URL)

		assets := []domain.AssetConfigs{}

		assets = append(assets, domain.AssetConfigs{
			Name:   fmt.Sprintf("%s-lambda", identifier),
			Source: "here in lambda",
			URL:    fmt.Sprintf("%s", respo.URL),
			DestPath:   "/opt/",
			Path:   "/opt/",
			SHA256: fn.Sha256,
			Executable: true,
			Unpack: false,
		})

 

	 
		provisionInstanceRequest := domain.ProvisionInstanceEvent{
			UserID: userID,
			Profile: "lambda",
			Name: fn.Name,
			ResourceID: fn.ID,
			Specs: domain.VMSpecs{
				CPU: 2,
				RAM: 2048,
				Storage: 15,
			},
			SessionID: c.GetString("requestID"),
			Assets: assets,
		}

		ec2Payload,_ := json.Marshal(provisionInstanceRequest)


		ec2ProvisionSubject := fmt.Sprintf("%s.ec2.task.provision", h.NatsPrefix)

		_, errr := h.Nats.Request(c.Request.Context(), ec2ProvisionSubject, ec2Payload, 5*time.Minute)
		if errr!=nil{
			fmt.Errorf("Failed to send the .provision request to ec2")
		}
	}()

	c.Stream(func(w io.Writer) bool {
		if msg, ok := <-eventChan; ok {
			if msg == "DONE" {
				log.Info("stream completed", zap.String("task_id", taskID))
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
				}
			}

			// Write raw JSON directly — no double encoding
			fmt.Fprintf(w, "data: %s\n\n", msg)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return true
		}
		return false
	})
}
