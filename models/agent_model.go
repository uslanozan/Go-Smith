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
}

// Orchestrator'ımızın /tools endpoint'inden dönen format
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}