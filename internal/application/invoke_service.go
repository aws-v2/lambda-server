package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lambda/internal/config"
	"lambda/internal/domain"
	"lambda/internal/domain/dto"
	"lambda/internal/domain/models"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/utils/logger"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type InvokeService struct {
	DB         *database.DB
	Nats       *event.NatsClient
	NatsPrefix string
	InvokeCfg  config.InvokeConfig
}

func NewInvokeService(db *database.DB, nats *event.NatsClient, natsPrefix string, invokeConfig config.InvokeConfig) *InvokeService {
	h := &InvokeService{DB: db, Nats: nats, NatsPrefix: natsPrefix, InvokeCfg: invokeConfig}

	return h
}

func (h *InvokeService) ResolveFunction(userId, functionId string) (*models.Function, error) {
	fn, err := h.DB.GetFunction(functionId, userId)
	if err != nil {
		return nil, err
	}
	return fn, nil
}

func (h *InvokeService) Invoke(c *context.Context, req dto.InvokeFunctionRequest) (*dto.BodyResponse, error) {
	// func (h *InvokeService) Invoke(c *gin.Context, req dto.InvokeFunctionRequest) (*dto.BodyResponse, error) {
	// cont := context.Background()
	log := req.Logger
	log = log.With(
		zap.String("function_identifier", req.FunctionId),
		zap.String("user_id", req.UserId),
	)

	fn, err := h.ResolveFunction(req.UserId, req.FunctionId)
	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
		return nil, fmt.Errorf("function not found or access denied")
	}

	startTime := time.Now()
	metric := models.LambdaMetric{
		FunctionName: fn.Name,
		UserID:       req.UserId,
		Status:       "success",
	}
	defer func() {
		metric.DurationMS = int(time.Since(startTime).Milliseconds())
		h.DB.RecordMetric(metric)
	}()

	taskID := req.TaskID
	log.Info("invocation started",
		zap.String("task_id", taskID),
		zap.String("function_name", fn.Name),
	)

	type result struct {
		resp *dto.BodyResponse
		err  error
	}
	resultChan := make(chan result, 1)

	go func() {
		downloadPresignUrl := fmt.Sprintf("%s.s3.task.create_presign_download_url", h.NatsPrefix)

		payload, err := json.Marshal(dto.CreatePresignDownloadURLRequest{
			UserID:        h.InvokeCfg.SystemUserId,
			CorrelationID: taskID,
			FileSha256:    fn.Sha256,
			AssetID:       fn.ID,
			FileCount:     1,
		})
		if err != nil {
			resultChan <- result{nil, fmt.Errorf("failed to marshal presign request: %w", err)}
			return
		}

		s3UrlRespData, err := h.Nats.Request(*c, downloadPresignUrl, payload, 5*time.Minute)
		if err != nil {
			resultChan <- result{nil, fmt.Errorf("nats presign request failed: %w", err)}
			return
		}

		var respo dto.CreatePresignDownloadURLResponse
		if err := json.Unmarshal(s3UrlRespData, &respo); err != nil {
			resultChan <- result{nil, fmt.Errorf("failed to unmarshal presign response: %w", err)}
			return
		}
		fmt.Printf("\n\n\nEvents payload::: %v\n\n\n", respo.URL)

		type LambdaContext struct {
			PythonVersion string    `json:"python_version"`
			InvokedAt     time.Time `json:"invoked_at"`
			MemoryLimit   int       `json:"memory_limit"`
			TimeOut       int       `json:"timeout"`
			FunctionId    string    `json:"function_id"`
			S3Url         string    `json:"s3_url"`
			UserId        string    `json:"user_id"`
			ApiKey        string    `json:"api_key"`
			Handler       string    `json:"handler"`
			Version       string    `json:"version"`
		}
		type LambdaInvokeRequest struct {
			Context LambdaContext  `json:"context"`
			Events  map[string]any `json:"events"`
		}
		eventsMap := make(map[string]any)
		eventsMap["divisor"] = 5
		eventsMap["players"] = [3]string{"martin", "agnes", "gloria"}
		functionUrlZip := fmt.Sprintf("%s://%s:8080%s", h.InvokeCfg.RuntimeProtocol, h.InvokeCfg.GatewayIP, respo.URL)

		fmt.Printf("\n\n:--: %v", functionUrlZip)

		invokeReq := LambdaInvokeRequest{
			Context: LambdaContext{
				InvokedAt:     time.Now(),
				TimeOut:       fn.TimeoutMS,
				UserId:        fn.UserID,
				Handler:       "main.start", // use the real handler, not a hardcoded "main.start"
				Version:       fn.Version,
				PythonVersion: "3",
				FunctionId:    fn.ID,
				MemoryLimit:   fn.MemoryMb,
				S3Url:         functionUrlZip,
				ApiKey:        h.InvokeCfg.InvokeApiKey,
			},
			Events: req.EventsPayload.(map[string]any),
		}

		jsonData, err := json.Marshal(invokeReq)
		if err != nil {
			resultChan <- result{nil, fmt.Errorf("could not marshal invoke request: %w", err)}
			return
		}

		provisionInstanceRequest := domain.ProvisionInstanceEvent{
			UserID:     req.UserId,
			Profile:    "lambda",
			Name:       fn.Name,
			ResourceID: fn.ID,
			Specs: domain.VMSpecs{
				CPU:     2,
				RAM:     fn.MemoryMb,
				Storage: 15,
			},
			SessionID:      uuid.New().String(),
			ForwardingPort: 9087,
		}

		ec2Payload, _ := json.Marshal(provisionInstanceRequest)

		ec2ProvisionSubject := fmt.Sprintf("%s.ec2.task.provision", h.NatsPrefix)

		ec2Responsee, errr := h.Nats.Request(*c, ec2ProvisionSubject, ec2Payload, 5*time.Minute)
		if errr != nil {
			fmt.Errorf("Failed to send the .provision request to ec2")
		}

		var ec2Response domain.EC2Response
		err = json.Unmarshal(ec2Responsee, &ec2Response)

		if errr != nil {
			fmt.Errorf("Failed to send the .provision request to ec2")
			return
		}

		if ec2Response.GatewayIP == "" || ec2Response.Code == 400 {
			fmt.Println("Gateway ip is blank , ie all lambda vms are in use ")
			fmt.Errorf("Gateway ip is blank , ie all lambda vms are in use ")
			return

		}

		fmt.Printf("\nEc2 response %+v\n", ec2Response)

		// ec2Response.GatewayIP = "10.0.5.169"
		ec2Response.GatewayPort = 9033
		// ec2Response.GatewayIP="localhost"
		functionUrl := fmt.Sprintf("%s://%s:%d/invoke", h.InvokeCfg.RuntimeProtocol, ec2Response.GatewayIP, ec2Response.GatewayPort)

		fmt.Printf("\n==>this %v this url: %v\n", ec2Response, functionUrl)
		httpReq, err := http.NewRequestWithContext(
			*c,
			http.MethodPost,
			functionUrl,
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			fmt.Printf("error calling invoke %v ", err)

			resultChan <- result{nil, fmt.Errorf("could not create request: %w", err)}
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-Api-Key", h.InvokeCfg.InvokeApiKey)

		client := &http.Client{}
		resp, err := client.Do(httpReq)
		if err != nil {
			resultChan <- result{nil, fmt.Errorf("runtime request failed: %w", err)}
			return
		}
		defer resp.Body.Close()
		fmt.Printf("invoke http request called")

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resultChan <- result{nil, fmt.Errorf("could not read runtime response: %w", err)}
			return
		}

		var lambdaResponse dto.BodyResponse
		if err := json.Unmarshal(body, &lambdaResponse); err != nil {
			resultChan <- result{nil, fmt.Errorf("could not unmarshal runtime response: %w", err)}
			return
		}

		type ReleaseVmRequest struct {
			VMId string `json:"vm_id"`
		}
		type RealeaseLambdaResponse struct {
			Code int `json:"code"`
		}

		ec2ReleaseVMPayload := ReleaseVmRequest{
			VMId: ec2Response.VMID,
		}

		releaseVMPayload, err := json.Marshal(ec2ReleaseVMPayload)
		if err != nil {
			fmt.Printf("failed to marshal release VM payload: %w", err)
		}

		releaseVMSubject := fmt.Sprintf(
			"%s.ec2.task.release_lambda",
			h.NatsPrefix,
		)

		releaseLambdaResponse, err := h.Nats.Request(
			*c,
			releaseVMSubject,
			releaseVMPayload,
			5*time.Minute,
		)
		if err != nil {
			fmt.Printf("failed to send release_lambda request to EC2: %w", err)
		}

		var lambdaReleaseData RealeaseLambdaResponse

		err = json.Unmarshal(releaseLambdaResponse, &lambdaReleaseData)
		if err != nil {
			fmt.Printf("failed to unmarshal release lambda response: %w", err)
		}

		fmt.Printf("Lambda release code: %v\n", lambdaReleaseData)
		resultChan <- result{&lambdaResponse, nil}
	}()

	anotherCont := *c
	select {
	case res := <-resultChan:
		if res.err != nil {
			metric.Status = "error"
			metric.ErrorMessage = res.err.Error()
			log.Warn("invocation failed",
				zap.String(logger.F.ErrorKind, "invocation_error"),
				zap.Error(res.err),
			)
			return nil, res.err
		}
		if res.resp != nil && res.resp.Code != 200 {
			metric.Status = "error"
		}
		return res.resp, nil

	case <-anotherCont.Done():
		metric.Status = "error"
		metric.ErrorMessage = "request cancelled or client disconnected"
		return nil, anotherCont.Err()
	}
}

/*

curl -X POST http://10.0.5.161:9033/invoklocalhostntent-Type: application/json"   -d '{
    "context": {
      "python_version": "3.11",
      "invoked_at": "2026-08-28T08:30:00Z",
      "memory_limit": 512,
      "timeout": 30,
	  "s3_url":"kllk"
      "s3_url": "http://localhost:8080/api/v1/s3/files/6df2422e-5c43-4853-8414-94a05b7c8a81/files/b30026fb-dcfa-4f91-afb4-0a58ed0b683a/download?t=eyJhIjoiOTFkNGUyOTctODBiMi00ZjI2LThiZWUtMTU4M2Q1YjIwZjhjIiwiYiI6IjZkZjI0MjJlLTVjNDMtNDg1My04NDE0LTk0YTA1YjdjOGE4MSIsImUiOjE3ODc4OTY4NTYsImsiOiI2ZGYyNDIyZS01YzQzLTQ4NTMtODQxNC05NGEwNWI3YzhhODEvZnVuY3Rpb25zLzkxZDRlMjk3LTgwYjItNGYyNi04YmVlLTE1ODNkNWIyMGY4YyIsIm0iOiJHRVQiLCJzaGEiOiJmZDkxNGVkZGE1Y2U0MTQ3ODRjYjBkZmIwZDQwMzZlMjIzZTUyMjI3ZWZlOTAxZTExNzM0MDYzODE5YWFkZWE4IiwidSI6ImJjYzY5OGI3LWY3OTItNDVjZS1hNGMyLTFlN2I0MDY3N2ExYiIsInVpZCI6IjE4NGU5MmNmLTE0YTgtNDZiYS1hYWIwLWRkNmE1NzQ1MjRhNCJ9.618cae5c51eface54b3aca687c8f1a8699bbbab62fa5d5147c7b9b3742525c88&token=eyJhbGciOiJIUzM4NCJ9.eyJhIjoiOTFkNGUyOTctODBiMi00ZjI2LThiZWUtMTU4M2Q1YjIwZjhjIiwidWlkIjoiMTg0ZTkyY2YtMTRhOC00NmJhLWFhYjAtZGQ2YTU3NDUyNGE0IiwiYiI6IjZkZjI0MjJlLTVjNDMtNDg1My04NDE0LTk0YTA1YjdjOGE4MSIsImUiOjE3ODc4OTY4NTYsInUiOiJiY2M2OThiNy1mNzkyLTQ1Y2UtYTRjMi0xZTdiNDA2NzdhMWIiLCJjb3JyZWxhdGlvbklkIjoiYTg1OTIzZmUtY2UxZC00MzUxLThhNjItZTU5OWZiN2EyY2NjIiwiayI6IjZkZjI0MjJlLTVjNDMtNDg1My04NDE0LTk0YTA1YjdjOGE4MS9mdW5jdGlvbnMvOTFkNGUyOTctODBiMi00ZjI2LThiZWUtMTU4M2Q1YjIwZjhjIiwibSI6IkdFVCIsInNoYSI6ImZkOTE0ZWRkYTVjZTQxNDc4NGNiMGRmYjBkNDAzNmUyMjNlNTIyMjdlZmU5MDFlMTE3MzQwNjM4MTlhYWRlYTgiLCJ1c2VySWQiOiIxODRlOTJjZi0xNGE4LTQ2YmEtYWFiMC1kZDZhNTc0NTI0YTQiLCJzdWIiOiIxODRlOTJjZi0xNGE4LTQ2YmEtYWFiMC1kZDZhNTc0NTI0YTQiLCJpYXQiOjE3ODc4OTU5NTYsImV4cCI6MTc4Nzk4MjM1Nn0.MEvVY-BerhPh-HvXmr4pVVxGOP0qwbE-Wzua0inUO2GHmiuouEd_O1mrLuVbfO-C",
      "user_id": "user123",
      "api_key": "test-api-key",
      "handler": "main.handler",
      "version": "1",
      "function_id": "function123"
    },
    "events": {
      "divisor": 3,
      "game_id": "game-123"
    }
  }'
*/
