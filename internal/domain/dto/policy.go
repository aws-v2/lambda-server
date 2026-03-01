package dto

type PolicyRequest struct {
	AccountID    string `json:"account_id" binding:"required"`
	PrincipalID  string `json:"principal_id" binding:"required"`
	ResourceType string `json:"resource_type" binding:"required"`
	ResourceID   string `json:"resource_id" binding:"required"`
	Action       string `json:"action" binding:"required"`
}

type PolicyEvent struct {
	RequestID    string      `json:"request_id"`
	PolicyID     string      `json:"policy_id,omitempty"`
	AccountID    string      `json:"account_id,omitempty"`
	PrincipalID  string      `json:"principal_id,omitempty"`
	ResourceType string      `json:"resource_type,omitempty"`
	ResourceID   string      `json:"resource_id,omitempty"`
	Action       string      `json:"action,omitempty"`
	Status       string      `json:"status,omitempty"`
	Message      string      `json:"message,omitempty"`
	Policy       interface{} `json:"policy,omitempty"`
	Error        string      `json:"error,omitempty"`
}
