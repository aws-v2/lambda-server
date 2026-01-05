package http

import (
	"lambda/internal/application"
	"lambda/internal/domain"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type LambdaHandlers struct {
	functionService   *application.FunctionService
	invocationService *application.InvocationService
}

func NewLambdaHandlers(functionService *application.FunctionService, invocationService *application.InvocationService) *LambdaHandlers {
	return &LambdaHandlers{
		functionService:   functionService,
		invocationService: invocationService,
	}
}

func (h *LambdaHandlers) CreateFunction(c *gin.Context) {
	var req CreateFunctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	runtime := domain.Runtime(req.Runtime)
	if !runtime.IsValid() {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid runtime. Supported: javascript, docker"})
		return
	}

	function, err := h.functionService.DeployFunction(c.Request.Context(), req.Name, userID.(string), runtime, req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ToCreateFunctionResponse(function))
}

func (h *LambdaHandlers) ListFunctions(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	functions, err := h.functionService.ListFunctions(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]FunctionResponse, 0, len(functions))
	for _, f := range functions {
		response = append(response, ToFunctionResponse(&f))
	}

	c.JSON(http.StatusOK, response)
}

func (h *LambdaHandlers) InvokeFunction(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "function name is required"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req InvokeFunctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	start := time.Now()
	result, err := h.invocationService.InvokeFunction(c.Request.Context(), name, userID.(string), req.Payload)
	duration := time.Since(start)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, InvokeFunctionResponse{
		Result:     result,
		DurationMs: duration.Milliseconds(),
	})
}

func (h *LambdaHandlers) DeleteFunction(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "function name is required"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	if err := h.functionService.DeleteFunction(c.Request.Context(), name, userID.(string)); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "function deleted successfully"})
}
