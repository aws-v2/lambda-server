package domain

import "encoding/json"

type EC2Response struct {
	GatewayIP   string `json:"gateway_ip"`
	GatewayPort int    `json:"gateway_port"`
}


type ProvisionInstanceEvent struct {
	UserID     string          `json:"userID"`
	Profile    string          `json:"profile" binding:"required"`
	Name       string          `json:"name" binding:"required"`
	ResourceID string          `json:"resource_id"`
	Specs      VMSpecs         `json:"specs" binding:"required"`
	SessionID  string          `json:"session_id"`
	Assets     []AssetConfigs  `json:"assets,omitempty"`
	Config     json.RawMessage `json:"config,omitempty"` // profile-specific config, opaque to the core service
}

type VMSpecs struct {
	CPU     int `json:"cpu" binding:"required"`
	RAM     int `json:"ram" binding:"required"` // MB
	Storage int `json:"storage,omitempty"`      // GB, optional override of image default
}
type AssetSource string

const (
	AssetSourceObject AssetSource = "object" // single presigned file (e.g. lambda binary)
	AssetSourceZip    AssetSource = "zip"    // presigned zip (folder/bucket export) — unpack after download
	AssetSourceInline AssetSource = "inline" // small payload embedded directly, base64
)

type AssetConfigs struct {
	Name       string      `json:"name"`
	Source     AssetSource `json:"source"`
	URL        string      `json:"url,omitempty"`
	InlineData string      `json:"inline_data,omitempty"` // only for AssetSourceInline
	DestPath   string      `json:"dest_path"`             // where the agent places/unpacks it
	SHA256     string      `json:"sha256,omitempty"`
	Unpack     bool        `json:"unpack,omitempty"`     // true = unzip after download
	Executable bool        `json:"executable,omitempty"` // chmod +x after placing

	Path string `json:"path"`
}