package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"net/url"
	"github.com/uslanozan/Go-Smith/models"
)

// Tüm agent'ları tutan ve yöneten merkezi registry
type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]models.AgentDefinition
}

type TaskRegistry struct {
	mu    sync.RWMutex
	tasks map[string]TaskInfo // Key: TaskID
}

// TaskInfo, bir görevin hangi agent'a ait olduğunu ve
// durum sorgulama adresini saklar.
type TaskInfo struct {
	AgentName          string
	AgentStatusBaseURL string // Örn: http://localhost:8082/task_status/
	AgentStopBaseURL   string // Örn: http://localhost:8082/task_stop/
}

// NewTaskRegistry, yeni, boş bir görev defteri oluşturur.
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{
		tasks: make(map[string]TaskInfo),
	}
}

// RegisterTask, yeni başlatılan bir asenkron görevi deftere kaydeder.
func (r *TaskRegistry) RegisterTask(taskID string, agent models.AgentDefinition) error {
	// Agent'ın ana "Endpoint" URL'sinden (http://.../create_event)
	// temel URL'sini (http://localhost:8082) çıkarmalıyız.
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

// GetTaskInfo, bir Task ID'ye karşılık gelen görev bilgilerini (sorgu URL'si) getirir.
func (r *TaskRegistry) GetTaskInfo(taskID string) (TaskInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.tasks[taskID]
	return info, ok
}

// Yeni bir kayıt defteri registry oluşturur ve başlatır
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]models.AgentDefinition),
	}
}

// Defterden agent'ın ismine göre arar, bulursa definition'ı döndürür
func (r *AgentRegistry) Get(name string) (models.AgentDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[name]
	return agent, ok
}


// Endpointi gizleyerek LLM'e agent listesini bildirir
// any = interface{} (tipini bilmediğimiz zamanlar)
func (r *AgentRegistry) GetToolsSpec() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	specs := make([]map[string]any, 0, len(r.agents))
	for _, agent := range r.agents {
		specs = append(specs, map[string]any{
			"name":        agent.Name,  // String
			"description": agent.Description,  // String
			"schema":      agent.Schema,  // json.RawMessage
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

// ---------------------- HELPER ----------------------

// Deftere yeni register kaydeder
func (r *AgentRegistry) register(def models.AgentDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[def.Name] = def
}
