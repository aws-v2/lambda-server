package dto

type LambdaScalingPolicyRequest struct {
	TenantID            string  `json:"tenant_id"`
	FunctionID          string  `json:"function_id"`
	MetricName          string  `json:"metric_name"`
	ScaleUpThreshold    float64 `json:"scale_up_threshold"`
	ScaleDownThreshold  float64 `json:"scale_down_threshold"`
	MaxConcurrencyLimit int     `json:"max_concurrency_limit"`
	MinConcurrencyLimit int     `json:"min_concurrency_limit"`
	ScaleStep           int     `json:"scale_step"`
	CooldownSeconds     int     `json:"cooldown_seconds"`
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
