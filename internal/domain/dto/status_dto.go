package dto

// ... existing structs ...

type NatsStatusMessage struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
