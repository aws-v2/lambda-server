package dto

type FunctionResponse struct {
	ID     string `json:"id"`
	ARN    string `json:"arn"`
	Sha256 string `json:"sha256"`

	Name        string `json:"name"`
	Description string `json:"description"`
	Runtime     string `json:"runtime"`
	Handler     string `json:"handler"`
	TimeoutMS   int    `json:"timeout_ms"`
	MemoryMb    int    `json:"memory_mb"`
	UploadURL string `json:"upload_url"`
}


type CreatePresignedURLResponse struct {
    UploadURL string `json:"upload_url"`
}