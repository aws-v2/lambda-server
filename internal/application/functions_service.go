package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"lambda/internal/domain/dto"
	"lambda/internal/domain/models"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/storage"
	"lambda/internal/utils/logger"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type FunctionService struct {
	DB         *database.DB
	Nats       *event.NatsClient
	Storage    *storage.Storage
	NatsPrefix string
	SystemUserID string

}

func NewLambdaService(
	db *database.DB,
	nats *event.NatsClient,
	stor *storage.Storage,
	natsPrefix string,
	systemUserID string,
) *FunctionService {
	return &FunctionService{
		DB:         db,
		Nats:       nats,
		Storage:    stor,
		NatsPrefix: natsPrefix,
		SystemUserID: systemUserID,
	}
}

func (h *FunctionService) ListFunctions(c *gin.Context, req dto.ListFunctionsRequest) ([]*dto.FunctionResponse, error) {
	log := req.Logger
	results, err := h.DB.ListFunctionsByUser(req.UserID)
	if err != nil {
		log.Error("failed to list functions",
			zap.String(logger.F.ErrorKind, "db_read_error"),
			zap.String("user_id", req.UserID),
			zap.Error(err),
		)
		return nil, err
	}

	log.Info("functions retrieved successfully",
		zap.String("user_id", req.UserID),
		zap.Int("count", len(results)),
	)

	var functions []*dto.FunctionResponse

	for _, function := range results {
		functions = append(functions, &dto.FunctionResponse{
			ID:          function.ID,
			ARN:         function.ARN,
			Sha256:      function.Sha256,
			Name:        function.Name,
			Description: function.Description,
			Runtime:     function.Runtime,
			Handler:     function.Handler,
			TimeoutMS:   function.TimeoutMS,
			MemoryMb:    function.MemoryMb,
		})

	}

	return functions, nil

}

func (h *FunctionService) GetFunction(c *gin.Context, req dto.GetFunctionRequest) (*dto.FunctionResponse, error) {
	log := req.Logger

	function, err := h.DB.GetFunction(req.FunctionID, req.UserID)

	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
		return nil, err
	}

	log.Info("function retrieved successfully", zap.String("function_name", function.Name))

	return &dto.FunctionResponse{
		ID:          function.ID,
		ARN:         function.ARN,
		Sha256:      function.Sha256,
		Name:        function.Name,
		Description: function.Description,
		Runtime:     function.Runtime,
		Handler:     function.Handler,
		TimeoutMS:   function.TimeoutMS,
		MemoryMb:    function.MemoryMb,
	}, nil
}

func CalculateSHA256Bytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (h *FunctionService) RegisterFunction(c *gin.Context, req dto.RegisterFunctionRequest) (*dto.FunctionResponse, error) {
	log := req.Logger

	file := req.FilePayload
	multipartFile, err := file.Open()
	payload, err := json.Marshal(multipartFile)
	if err != nil {
		log.Warn("missing file for non-image function",
			zap.String(logger.F.ErrorKind, "invalid_request"),
		)
		return nil, err
	}

	filePayload := payload

	openedFile, err := file.Open()
	if err != nil {
		log.Error("failed to open uploaded file",
			zap.String(logger.F.ErrorKind, "file_open_error"),
			zap.Error(err),
		)
		return nil, err
	}

	defer openedFile.Close()

	filesha := CalculateSHA256Bytes(filePayload)
	ext := filepath.Ext(file.Filename)
	isZip := ext == ".zip" ||
		file.Header.Get("Content-Type") == "application/zip" ||
		file.Header.Get("Content-Type") == "application/x-zip-compressed"
	functionID := uuid.New().String()
	var s3ploadUrl string

	if isZip {
		functionUploadUrl, err := h.Storage.SaveFunctionZip(c, req.Name, openedFile, file.Size, functionID, payload, filesha, h.SystemUserID)
		if err != nil {
			log.Error("failed to save function zip",
				zap.String(logger.F.ErrorKind, "storage_error"),
				zap.Error(err),
			)
			return nil, err
		}
		s3ploadUrl = functionUploadUrl
		log.Info("function zip saved", zap.String("upload url", functionUploadUrl))
	} else {
		functionUploadUrl, err := h.Storage.SaveFunctionBinary(c, req.Name, openedFile, functionID, payload, filesha,h.SystemUserID)
		if err != nil {
			log.Error("failed to save function binary",
				zap.String(logger.F.ErrorKind, "storage_error"),
				zap.Error(err),
			)
			return nil, err
		}
		s3ploadUrl = functionUploadUrl
		log.Info("function binary saved", zap.String("upload url", functionUploadUrl))
	}

	arn := fmt.Sprintf("arn:serwin:lambda:%s:%s:function:%s", req.Region, req.UserId, req.Name)
	if req.Region == "" {
		req.Region = "eu-north-1"
	}

	errw := h.DB.SaveFunction(models.Function{
		ID:          functionID,
		Name:        req.Name,
		ARN:         arn,
		UserID:      req.UserId,
		Description: req.Description,
		Sha256:      filesha,
		Region:      req.Region,
		Version:     req.Version,

		Runtime:   req.Runtime,
		TimeoutMS: req.TimeoutMS,
		Handler:   req.Handler,
		MemoryMb:  req.MemoryMb,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if errw != nil {
		log.Error("failed to save function metadata",
			zap.String(logger.F.ErrorKind, "db_write_error"),
			zap.Error(err),
		)
		return nil, errw
	}

	log.Info("function registered successfully", zap.String("function_arn", arn))

	return &dto.FunctionResponse{
		ID:          functionID,
		Name:        req.Name,
		ARN:         arn,
		Description: req.Description,
		Sha256:      filesha,
		UploadURL:   s3ploadUrl,

		Runtime:   req.Runtime,
		TimeoutMS: req.TimeoutMS,
		Handler:   req.Handler,
		MemoryMb:  req.MemoryMb,
	}, nil

}
