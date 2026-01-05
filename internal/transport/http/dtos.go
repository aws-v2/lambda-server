package http

import (
	"lambda/internal/domain"
	"time"
)

type CreateFunctionRequest struct {
	Name    string `json:"name" binding:"required"`
	Runtime string `json:"runtime" binding:"required"`
	Code    string `json:"code" binding:"required"`
}

type CreateFunctionResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Runtime   string `json:"runtime"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type FunctionResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Runtime   string `json:"runtime"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	MemoryMB  int    `json:"memoryMb"`
	TimeoutMs int    `json:"timeoutMs"`
}

type InvokeFunctionRequest struct {
	Payload map[string]interface{} `json:"payload"`
}

type InvokeFunctionResponse struct {
	Result     interface{} `json:"result"`
	DurationMs int64       `json:"durationMs"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func ToFunctionResponse(f *domain.Function) FunctionResponse {
	return FunctionResponse{
		ID:        f.ID,
		Name:      f.Name,
		Runtime:   string(f.Runtime),
		Status:    string(f.Status),
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
		UpdatedAt: f.UpdatedAt.Format(time.RFC3339),
		MemoryMB:  f.MemoryMB,
		TimeoutMs: f.TimeoutMs,
	}
}

func ToCreateFunctionResponse(f *domain.Function) CreateFunctionResponse {
	return CreateFunctionResponse{
		ID:        f.ID,
		Name:      f.Name,
		Runtime:   string(f.Runtime),
		Status:    string(f.Status),
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
	}
}
