package handlers

import (
	"net/http"
	"strings"

	"lambda/internal/infrastructure/database"
	"lambda/internal/utils/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type MetricHandler struct {
	DB                *database.DB
	ResolveFunction   func(identifier, userID string) (*database.Function, error)
	ResolveIdentifier func(c *gin.Context) string
}

func NewMetricHandler(db *database.DB) *MetricHandler {
	h := &MetricHandler{DB: db}

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

func (h *MetricHandler) GetMetrics(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.metrics"),
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

	log.Info("metrics retrieved successfully", zap.String("function_name", fn.Name))

	c.JSON(http.StatusOK, metrics)
}