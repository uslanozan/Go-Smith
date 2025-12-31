package models

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// LLM'den Orchestrator'a gelecek olan JSON isteğinin formatıdır.
type OrchestratorTaskRequest struct {
	AgentName string          `json:"agent_name"`
	Arguments json.RawMessage `json:"arguments"`
}

// --------- ASENKRON GÖREVLER İÇİN ---------

type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

// Task başlatılır
type TaskStartResponse struct {
	TaskID string     `json:"task_id"` //! Merkezi bir yerden dağıtılmadığı için (DB gibi) int ve AI yapmak sıkıntı
	Status TaskStatus `json:"status"`
}

// Bir görevin /task_status/:id endpoint'inden sorgulandığında döndürülecek "Durum Raporu"dur.
type TaskStatusResponse struct {
	TaskID string          `json:"task_id"`
	Status TaskStatus      `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (TaskStatus) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{
			StatusPending,
			StatusRunning,
			StatusCompleted,
			StatusFailed,
		},
	}
}
