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
