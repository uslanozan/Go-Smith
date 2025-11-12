package models

import (
	"encoding/json"
)

// LLM'den Orchestrator'a gelecek olan JSON isteğinin formatıdır.
type OrchestratorTaskRequest struct {
	AgentName string          `json:"agent_name"`
	Arguments json.RawMessage `json:"arguments"`
}