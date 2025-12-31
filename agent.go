package main

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"sync"

	"github.com/uslanozan/Go-Smith/models"
)

// Tüm agent'ları tutan ve yöneten merkezi registry
type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]models.AgentDefinition
}

type TaskRegistry struct {
	mu    sync.RWMutex
	tasks map[string]TaskInfo
}

// TaskInfo, bir görevin hangi agent'a ait olduğunu ve durum sorgulama adresini saklar.
type TaskInfo struct {
	AgentName          string
	AgentStatusBaseURL string
	AgentStopBaseURL   string
}

// NewTaskRegistry, yeni, boş bir görev defteri oluşturur.
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{
		tasks: make(map[string]TaskInfo),
	}
}

func (r *TaskRegistry) RegisterTask(taskID string, agent models.AgentDefinition) error {
	base, err := url.Parse(agent.Endpoint)
	if err != nil {
		return err
	}

	statusURL := base.ResolveReference(&url.URL{Path: agent.StatusEndpointPath})

	stopURL := base.ResolveReference(&url.URL{Path: agent.StopEndpointPath})

	info := TaskInfo{
		AgentName:          agent.Name,
		AgentStatusBaseURL: statusURL.String(),
		AgentStopBaseURL:   stopURL.String(),
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[taskID] = info
	log.Printf("Görev deftere kaydedildi: TaskID %s -> Agent %s", taskID, info.AgentName)
	return nil
}

func (r *TaskRegistry) GetTaskInfo(taskID string) (TaskInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.tasks[taskID]
	return info, ok
}

func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]models.AgentDefinition),
	}
}

func (r *AgentRegistry) Get(name string) (models.AgentDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[name]
	return agent, ok
}

func (r *AgentRegistry) GetToolsSpec() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	specs := make([]map[string]any, 0, len(r.agents))
	for _, agent := range r.agents {
		specs = append(specs, map[string]any{
			"name":        agent.Name,
			"description": agent.Description,
			"schema":      agent.Schema,
		})
	}
	return specs
}

// Orchestrator ilk başladığında config/agents.json'ı okuyarak agent'ları deftere otomatik kaydeder
func LoadAgentsFromConfig(registry *AgentRegistry, configFile string) error {
	log.Printf("Agent konfigrasyonu yükleniyor: %s", configFile)

	file, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var definitions []models.AgentDefinition
	if err := json.NewDecoder(file).Decode(&definitions); err != nil {
		return err
	}

	count := 0
	for _, def := range definitions {
		registry.register(def)
		count++
	}

	log.Printf("%d agent eylemi başarıyla yüklendi.", count)
	return nil
}

// ---------------------- HELPERS ----------------------

func (r *AgentRegistry) register(def models.AgentDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[def.Name] = def
}
