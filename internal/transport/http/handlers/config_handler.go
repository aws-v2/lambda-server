package handlers

import (
	"io"
	"net/http"

	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/auth"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/storage"
	"lambda/internal/utils/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ConfigHandler struct {
	DB *database.DB
	Storage *storage.Storage
	Resolver *auth.ApiKeyResolver
	Region string
	NatsClient *event.NatsClient
	ResolveFunction func(identifier, userID string) (*database.Function, error)
	ResolveIdentifier func(c *gin.Context) string
	
}

func NewConfigHandler(db *database.DB, storage *storage.Storage, resolver *auth.ApiKeyResolver, region string, natsClient *event.NatsClient) *ConfigHandler {
	return &ConfigHandler{DB: db, Storage: storage, Resolver: resolver, Region: region, NatsClient: natsClient}
}

func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.update_config"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := h.ResolveIdentifier(c)
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

	fn, err := h.ResolveFunction(identifier, userIDStr)
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

	log.Info("function configuration updated successfully", zap.String("function_name", fn.Name))

	c.JSON(http.StatusOK, gin.H{"message": "configuration updated successfully"})
}

func (h *ConfigHandler) UpdateCode(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.update_code"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := h.ResolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

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

	log.Info("function code updated successfully", zap.String("function_name", fn.Name))

	c.JSON(http.StatusOK, gin.H{"message": "code updated successfully"})
}

func (h *ConfigHandler) GetCode(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.get_code"),
		zap.String(logger.F.Domain, "lambda"),
	)

	identifier := h.ResolveIdentifier(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

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
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found or access denied"})
		return
	}

	content, err := h.Storage.ReadFunctionFile(fn.Name, "handler")
	if err != nil {
		log.Error("failed to read code artifact",
			zap.String(logger.F.ErrorKind, "storage_error"),
			zap.String("function_name", fn.Name),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read code artifact"})
		return
	}

	log.Info("function code retrieved successfully", zap.String("function_name", fn.Name))

	c.Data(http.StatusOK, "text/plain", content)
}