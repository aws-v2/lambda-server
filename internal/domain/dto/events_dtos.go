package dto


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



type LambdaScalingPolicyEvent struct {
	RequestID string                     `json:"request_id"`
	Action    string                     `json:"action"` // create, update, delete, list
	Policy    LambdaScalingPolicyRequest `json:"policy,omitempty"`
	Token     string                     `json:"token,omitempty"`
	Status    string                     `json:"status,omitempty"`
	Message   string                     `json:"message,omitempty"`
	Error     string                     `json:"error,omitempty"`
	Data      any                        `json:"data,omitempty"`
}

type LambdaScaleEvent struct {
	TenantID   string  `json:"tenant_id"`
	FunctionID string  `json:"function_id"`
	Reason     string  `json:"reason"`
	Metric     string  `json:"metric"`
	Value      float64 `json:"value"`
	Action     string  `json:"action"` // INCREASE_PROVISIONED_CONCURRENCY or DECREASE_PROVISIONED_CONCURRENCY
}



type BodyResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}