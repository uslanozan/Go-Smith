package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/uslanozan/Go-Smith/models"
)

type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]models.AgentDefinition
}

type TaskRegistry struct {
	mu    sync.RWMutex
	tasks map[string]TaskInfo
}

type TaskInfo struct {
	AgentName          string
	AgentStatusBaseURL string
	AgentStopBaseURL   string
}

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

func (r *AgentRegistry) register(def models.AgentDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[def.Name] = def
}

func (r *AgentRegistry) clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents = make(map[string]models.AgentDefinition)
}

// ---------------------- LOADERS ----------------------

// LoadAgents: Ortam değişkenine göre API veya Dosya üzerinden yükleme yapar
func LoadAgents(registry *AgentRegistry) error {
	source := os.Getenv("AGENT_SOURCE") // "http" ise API'ye gider
	backendURL := os.Getenv("BACKEND_AGENTS_URL")

	if source == "http" && backendURL != "" {
		log.Printf("🌐 Ajanlar API üzerinden yükleniyor: %s", backendURL)
		err := LoadAgentsFromAPI(registry, backendURL)
		if err == nil {
			return nil
		}
		log.Printf("⚠️ API hatası, yerel dosyaya dönülüyor: %v", err)
	}

	return LoadAgentsFromConfig(registry, "config/agents.json")
}

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

	for _, def := range definitions {
		registry.register(def)
	}
	log.Printf("%d agent eylemi başarıyla yüklendi.", len(definitions))
	return nil
}

// LoadAgentsFromAPI: Silka Backend'den (snake_case) verileri çeker ve AgentDefinition'a mapler
func LoadAgentsFromAPI(registry *AgentRegistry, url string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Silka-Backend'den gelen ham yapıyı karşılamak için yerel bir tip (Mapping için)
	type apiAgent struct {
		AgentName      string          `json:"agent_name"`
		Description    string          `json:"description"`
		Endpoint       string          `json:"endpoint"`
		StopEndpoint   string          `json:"stop_endpoint"`
		StatusEndpoint string          `json:"status_endpoint"`
		SchemaJSON     json.RawMessage `json:"schema_json"`
		Status         int             `json:"status"`
	}

	var dbAgents []apiAgent
	if err := json.NewDecoder(resp.Body).Decode(&dbAgents); err != nil {
		return err
	}

	registry.clear() // Refresh sırasında mükerrer kayıt olmaması için
	count := 0
	for _, da := range dbAgents {
		if da.Status == 0 {
			continue
		}

		// API verisini AgentDefinition modeline mapliyoruz
		def := models.AgentDefinition{
			Name:               da.AgentName,
			Description:        da.Description,
			Endpoint:           da.Endpoint,
			StatusEndpointPath: da.StatusEndpoint,
			StopEndpointPath:   da.StopEndpoint,
			Schema:             da.SchemaJSON,
		}

		// Varsayılan path'leri kontrol et
		if def.StatusEndpointPath == "" {
			def.StatusEndpointPath = "/task_status/"
		}
		if def.StopEndpointPath == "" {
			def.StopEndpointPath = "/task_stop/"
		}

		registry.register(def)
		count++
	}

	log.Printf("🌐 API üzerinden %d agent başarıyla yüklendi.", count)
	return nil
}
