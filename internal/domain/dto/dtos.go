package dto

// Shared Structs

type ExecutionDetails struct {
	Kind    string   `json:"kind" form:"execution.kind"`
	Path    string   `json:"path" form:"execution.path"`
	Command []string `json:"command" form:"execution.command"`
}

type ResourceDetails struct {
	CPU    int `json:"cpu" form:"resources.cpu"`
	Memory int `json:"memory" form:"resources.memory"`
}

// Requests

type InvokeRequest struct {
	Name    string                 `json:"name" binding:"required"`
	Payload map[string]interface{} `json:"payload"`
}

type RegisterFunctionRequest struct {
	Name        string            `form:"name" binding:"required"`
	Type        string            `form:"type"`
	Image       string            `form:"image"`
	Execution   ExecutionDetails  `form:"execution"`
	Resources   ResourceDetails   `form:"resources"`
	Env         map[string]string `form:"env"`
	TimeoutMS   int               `form:"timeout_ms"`
	Description string            `form:"description"`
}

type UpdateFunctionConfigRequest struct {
	Memory      int    `json:"memory" binding:"required"`
	Timeout     int    `json:"timeout" binding:"required"`
	Description string `json:"description"`
}

// NATS Message

type NatsMessage struct {
	TraceID   string            `json:"trace_id"`
	TaskID    string            `json:"task_id"`
	Type      string            `json:"type"`
	Image     string            `json:"image"`
	Execution ExecutionDetails  `json:"execution"`
	Resources ResourceDetails   `json:"resources"`
	Env       map[string]string `json:"env"`
}
