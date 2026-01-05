package domain

import "time"

type Invocation struct {
	ID           string          `json:"id"`
	FunctionID   string          `json:"functionId"`
	FunctionName string          `json:"functionName"`
	Status       InvocationStatus `json:"status"`
	Payload      string          `json:"payload"`
	Result       string          `json:"result"`
	Error        string          `json:"error,omitempty"`
	StartedAt    time.Time       `json:"startedAt"`
	CompletedAt  *time.Time      `json:"completedAt,omitempty"`
	DurationMs   int64           `json:"durationMs"`
}

type InvocationStatus string

const (
	InvocationStatusRunning   InvocationStatus = "RUNNING"
	InvocationStatusSuccess   InvocationStatus = "SUCCESS"
	InvocationStatusFailed    InvocationStatus = "FAILED"
	InvocationStatusTimeout   InvocationStatus = "TIMEOUT"
)
