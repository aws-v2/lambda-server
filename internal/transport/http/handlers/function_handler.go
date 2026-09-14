package handlers

import (
	"fmt"
	"net/http"

	"lambda/internal/domain/dto"
	"lambda/internal/utils"
	"lambda/internal/utils/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"lambda/internal/application"
)

type LambdaHandlers struct {
	FunctionService application.FunctionService
}

func NewLambdaHandlers(
	funcService application.FunctionService,

) *LambdaHandlers {
	return &LambdaHandlers{
		FunctionService: funcService,
	}
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
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("failed to bind register request"))

		return
	}

	userID := c.GetString("userId")

	log = log.With(
		zap.String("function_name", req.Name),
		zap.String("user_id", userID),
	)

	req.Logger = log
	req.UserId = userID
	req.TimeoutMS=200
	req.Version ="1.0"

	file, err := c.FormFile("file")
	if err != nil {
		log.Warn("failed to bind register request",
			zap.String(logger.F.ErrorKind, "invalid_request"),
			zap.Error(err),
		)
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("failed to bind register request"))

		return

	}

	req.FilePayload = file
	req.Handler="main.start"
	req.Runtime="python 3.11"
	req.MemoryMb=258


	fn, err := h.FunctionService.RegisterFunction(c, req)

	if err != nil {
		log.Warn("failed to  register function",
			zap.String(logger.F.ErrorKind, "invalid_request"),
			zap.Error(err),
		)
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("function could not be saved succesfully"))

		return
	}

	log.Info("function registered successfully", zap.String("function_arn", fn.ARN))

	utils.RespondSucces(c, http.StatusCreated, "function registered successfully", fn)

}

func (h *LambdaHandlers) ListFunctions(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.list"),
		zap.String(logger.F.Domain, "lambda"),
	)

	userID := c.GetString("userId")
	requestID := c.GetString("requestID")

	req := dto.ListFunctionsRequest{
		UserID: userID,
		Logger: log,
	}

	functions, err := h.FunctionService.ListFunctions(c, req)

	if err != nil {
		log.Error("failed to list functions",
			zap.String(logger.F.ErrorKind, "db_read_error"),
			zap.String("requestID", requestID),
			zap.Error(err),
		)
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("function could not be fetched succesfully"))
		return
	}

	log.Info("functions retrieved successfully",
		zap.String("requestID", requestID),
		zap.Int("count", len(functions)),
	)
	utils.RespondSucces(c, http.StatusOK, "succesfully retrieved all functions", functions)

}

func (h *LambdaHandlers) GetFunction(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.get"),
		zap.String(logger.F.Domain, "lambda"),
	)

	userID := c.GetString("userId")
	functionId := c.Param("name")

	var req dto.GetFunctionRequest
	if err := c.ShouldBind(&req); err != nil {
		log.Warn("failed to bind register request",
			zap.String(logger.F.ErrorKind, "invalid_request"),
			zap.Error(err),
		)
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("failed to bind register request"))

		return
	}

	req = dto.GetFunctionRequest{
		UserID: userID,
		Logger: log,
		FunctionID: functionId,
	}

	fmt.Printf("\n\n-: %v\n\n", req)

	fn, err := h.FunctionService.GetFunction(c, req)
	if err != nil {
		log.Warn("function lookup failed",
			zap.String(logger.F.ErrorKind, "not_found"),
			zap.Error(err),
		)
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("function not found or access denied"))
		return
	}

	log.Info("function retrieved successfully", zap.String("function_name", fn.Name))
	utils.RespondSucces(c, http.StatusOK, "succesfully fetched the function", fn)

}
