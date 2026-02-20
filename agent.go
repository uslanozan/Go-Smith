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

// 🔥 GÜNCELLENDİ: Artık Ajanları değil, Ajanların altındaki Fonksiyonları listeliyor
func (r *AgentRegistry) GetToolsSpec() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var specs []map[string]any
	for _, agent := range r.agents {
		for _, f := range agent.Functions {
			specs = append(specs, map[string]any{
				"name":        f.FunctionName,
				"description": f.Description,
				"schema":      f.Schema,
			})
		}
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

func LoadAgents(registry *AgentRegistry) error {
	source := os.Getenv("AGENT_SOURCE")
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

// 🔥 GÜNCELLENDİ: Silka-Backend'den gelen yeni Hiyerarşik JSON yapısını karşılayacak şekilde değiştirildi
func LoadAgentsFromAPI(registry *AgentRegistry, url string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// İç içe JSON yapısını karşılayacak yerel struct'lar
	type apiFunction struct {
		FunctionName        string          `json:"function_name"`
		FunctionDescription string          `json:"function_description"`
		APIPath             string          `json:"api_path"`
		SchemaJSON          json.RawMessage `json:"schema_json"`
	}

	type apiAgent struct {
		AgentName      string        `json:"agent_name"`
		Description    string        `json:"description"`
		Endpoint       string        `json:"endpoint"`
		StopEndpoint   string        `json:"stop_endpoint"`
		StatusEndpoint string        `json:"status_endpoint"`
		Status         int           `json:"status"`
		Functions      []apiFunction `json:"functions"` // 🔥 Yeni eklenen dizi
	}

	var dbAgents []apiAgent
	if err := json.NewDecoder(resp.Body).Decode(&dbAgents); err != nil {
		return err
	}

	registry.clear()
	count := 0

	for _, da := range dbAgents {
		if da.Status == 0 {
			continue
		}

		// API'den gelen fonksiyonları Go-Smith modeline dönüştür (Map işlemi)
		var mappedFuncs []models.AgentFunction
		for _, f := range da.Functions {
			mappedFuncs = append(mappedFuncs, models.AgentFunction{
				FunctionName: f.FunctionName,
				Description:  f.FunctionDescription,
				APIPath:      f.APIPath,
				Schema:       f.SchemaJSON,
			})
		}

		// API verisini AgentDefinition modeline mapliyoruz
		def := models.AgentDefinition{
			Name:               da.AgentName,
			Description:        da.Description,
			Endpoint:           da.Endpoint,
			StatusEndpointPath: da.StatusEndpoint,
			StopEndpointPath:   da.StopEndpoint,
			Functions:          mappedFuncs, // 🔥 Maplediğimiz fonksiyonları atıyoruz
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

// İstenen fonksiyonun hangi ajana ait olduğunu bulur
func (r *AgentRegistry) GetByFunction(functionName string) (models.AgentDefinition, models.AgentFunction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, agent := range r.agents {
		for _, f := range agent.Functions {
			if f.FunctionName == functionName {
				return agent, f, true
			}
		}
	}
	return models.AgentDefinition{}, models.AgentFunction{}, false
}