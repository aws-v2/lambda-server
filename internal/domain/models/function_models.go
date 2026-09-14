package models

import "time"

type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,
		SSLMode:         "require",
	}
}

type ExecutionDetails struct {
	Kind    string   `json:"kind"`
	Path    string   `json:"path"`
	Command []string `json:"command"`
}

type ResourceDetails struct {
	CPU    int `json:"cpu"`
	Memory int `json:"memory"`
}

type Function struct {
	ID                     string `json:"id"`
	Name                   string
	ARN                    string
	UserID                 string
	Type                   string
	Image                  string
	Execution              ExecutionDetails
	Resources              ResourceDetails
	Env                    map[string]string
	Description            string
	ProvisionedConcurrency int
	Sha256                 string
	Version                string
	Region                 string    `json:"region"`
	Runtime                string    `json:"runtime"`
	Handler                string    `json:"handler"`
	TimeoutMS              int       `json:"timeout"`
	MemoryMb               int       `json:"memory_mb"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type LambdaMetric struct {
	FunctionName string
	UserID       string
	DurationMS   int
	Status       string // 'success', 'error'
	ErrorMessage string
	Timestamp    time.Time
}

type TimelinePoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

type LambdaMetricsResponse struct {
	Invocations int             `json:"invocations"`
	Duration    float64         `json:"duration"` // Avg duration as float64
	Errors      int             `json:"errors"`
	Timeline    []TimelinePoint `json:"timeline"`
}

type ApiKey struct {
	AccessKeyID   string    `json:"access_key_id"`
	UserID        string    `json:"user_id"`
	SecretKeyHash string    `json:"secret_key_hash"`
	Enabled       bool      `json:"enabled"`
	LastSynced    time.Time `json:"last_synced"`
}
