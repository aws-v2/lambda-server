package domain

import "time"

type Function struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"ownerId"`
	Runtime   Runtime   `json:"runtime"`
	Code      string    `json:"code"`
	Status    FunctionStatus `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	MemoryMB  int       `json:"memoryMb"`
	TimeoutMs int       `json:"timeoutMs"`
}

type FunctionStatus string

const (
	FunctionStatusActive   FunctionStatus = "ACTIVE"
	FunctionStatusInactive FunctionStatus = "INACTIVE"
	FunctionStatusFailed   FunctionStatus = "FAILED"
)
