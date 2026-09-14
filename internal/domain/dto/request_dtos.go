package dto

import (
	"mime/multipart"

	"go.uber.org/zap"
)

// Requests

type InvokeRequest struct {
	Name    string                 `json:"name" binding:"required"`
	Payload map[string]interface{} `json:"payload"`
}

type RegisterFunctionRequest struct {
	Name        string                `form:"name" binding:"required"`
	UserId      string                `json:"user_id"`
	Description string                `form:"description"`
	Runtime     string                `form:"runtime"`
	Region      string                `form:"region"`
	Handler     string                `form:"handler"`
	TimeoutMS   int                   `form:"timeout_ms"`
	MemoryMb    int                   `form:"memory_mb"`
	Logger      *zap.Logger           `json:"logger"`
	FilePayload *multipart.FileHeader `json:"file_payload"`
	Version     string                `json:"version"`
}

type UpdateFunctionConfigRequest struct {
	Memory      int    `json:"memory" binding:"required"`
	Timeout     int    `json:"timeout" binding:"required"`
	Description string `json:"description"`
	ConfigId    string `json:"config_id"`
	UserId      string `json:"user_id"`
	RequestId   string `json:"request_id"`
	FunctionID  string `json:"function_id"`
}

type PolicyRequest struct {
	AccountID    string `json:"account_id" binding:"required"`
	PrincipalID  string `json:"principal_id" binding:"required"`
	ResourceType string `json:"resource_type" binding:"required"`
	ResourceID   string `json:"resource_id" binding:"required"`
	Action       string `json:"action" binding:"required"`
}

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

type ListFunctionsRequest struct {
	UserID string      `json:"user_id"`
	Logger *zap.Logger `json:"logger"`
}

type GetFunctionRequest struct {
	UserID     string      `json:"user_id"`
	FunctionID string      `json:"function_id"`
	Logger     *zap.Logger `json:"logger"`
}

type GetCodeRequest struct {
	UserID     string `json:"user_id"`
	RequestID  string `json:"request_id"`
	FunctionID string `json:"function_id"`
}

type UpdateCodeRequest struct {
	UserID     string `json:"user_id"`
	RequestID  string `json:"request_id"`
	FunctionID string `json:"function_id"`
}

type CreatePresignUploadURLRequest struct {

}
type CreatePresignDownloadURLRequest struct {
	UserID        string `json:"user_id"`
	CorrelationID string `json:"correlation_id"`
	FileSha256    string `json:"sha256"`
	AssetID       string `json:"asset_id"`
	FileCount     int    `json:"file_count"`
}
type CreatePresignDownloadURLResponse struct {
	URL string `json:"url"`
}

type AssetConfigs struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type GameManifest struct {
	Parameters map[string]string `json:"parameters"`
}

type InvokeFunctionRequest struct {
	UserId    string      `json:"user_id"`
	RequestId string      `json:"request_id"`
	Logger    *zap.Logger `json:"logger"`
	TaskID string `json:"task_id"`
	EventsPayload interface{} `json:"events_payload"`
	FunctionId string `json:"function_id"`
}

type AssetType string

type CreatePresignedURLRequest struct {
    UserID     string    `json:"user_id"`
    FunctionID     string    `json:"game_id,omitempty"`
    AssetID    string    `json:"asset_id"`
    AssetType  AssetType `json:"asset_type"`  // "game" | "template"
    AssetName  string    `json:"asset_name"`  // this is the nameof the job/game/render job this asset belongs to like kalshi or ruto tracker
    BucketName string    `json:"bucket_name"` // this is the nameof the job/game/render job this asset belongs to like kalshi or ruto tracker
    Key        string    `json:"key"`

    Sha256 string `json:"sha256"`
}