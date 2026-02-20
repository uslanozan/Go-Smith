package models

import (
	"encoding/json"
)

// 🔥 YENİ: Ajanın yeteneklerini temsil eden yapı
type AgentFunction struct {
	FunctionName string          `json:"function_name"`
	Description  string          `json:"description"`
	APIPath      string          `json:"api_path"`
	Schema       json.RawMessage `json:"schema"`
}

type AgentDefinition struct {
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Endpoint           string          `json:"endpoint"`
	StatusEndpointPath string          `json:"status_endpoint_path,omitempty"`
	StopEndpointPath   string          `json:"stop_endpoint_path,omitempty"`
	
	// Eski "Schema json.RawMessage" SİLİNDİ
	// Yerine bu geldi:
	Functions          []AgentFunction `json:"functions"`
}

type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}