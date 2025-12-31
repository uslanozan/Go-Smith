package models

import (
	"encoding/json"
)

type AgentDefinition struct {
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Schema             json.RawMessage `json:"schema"`
	Endpoint           string          `json:"endpoint"`
	StatusEndpointPath string          `json:"status_endpoint_path,omitempty"`
	StopEndpointPath   string          `json:"stop_endpoint_path,omitempty"`
}

type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}
