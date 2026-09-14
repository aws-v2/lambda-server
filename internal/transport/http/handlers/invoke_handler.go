package handlers

import (
	// "encoding/json"
	// "fmt"
	// "io"
	// "lambda/internal/domain/models"

	"lambda/internal/application"
	"lambda/internal/domain/dto"
	"lambda/internal/utils"
	"lambda/internal/utils/logger"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type InvokeHandler struct {
	InvokeService *application.InvokeService
}

func NewInvokeHandler(invokeService *application.InvokeService) *InvokeHandler {
	h := &InvokeHandler{InvokeService: invokeService}

	return h
}
func (h *InvokeHandler) Invoke(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.invoke"),
		zap.String(logger.F.Domain, "lambda"),
	)
	var sreq dto.InvokeFunctionRequest

	if err := c.ShouldBindJSON(&sreq); err != nil {
		log.Warn("missing identifier in request",
			zap.String(logger.F.ErrorKind, "invalid_request"),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not bind data to handler"})
		return
	}

	sreq.FunctionId = c.Param("name")
	sreq.Logger = log
	sreq.UserId = c.GetString("userId")
	sreq.RequestId = c.GetString("requestId")
	sreq.TaskID = uuid.New().String()
	cont := c.Request.Context() 



	invokeResp, err := h.InvokeService.Invoke(&cont, sreq)
	if err != nil {
		log.Warn("invoke service failed",
			zap.String(logger.F.ErrorKind, "invocation_failed"),
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Could not invoke function: " + err.Error()})
		return
	}

	utils.RespondSucces(c, http.StatusOK, "function invoked successfully", invokeResp)
}
