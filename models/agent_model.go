package models

import (
	"encoding/json"
)

// AgentDefinition, config/agents.json'daki agentlara karşılık gelir
type AgentDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
	Endpoint    string          `json:"endpoint"`
	StatusEndpointPath string `json:"status_endpoint_path,omitempty"`  // Agent asenkronsa görev durumunu sorgulama yolunu belirtir
	StopEndpointPath   string `json:"stop_endpoint_path,omitempty"` // Stop endpointi
}

// Orchestrator'ımızın /tools endpoint'inden dönen format
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}