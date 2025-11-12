package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/uslanozan/Ollama-the-Agent/models"
)

// Tüm agent'ları tutan ve yöneten merkezi registry
type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]models.AgentDefinition
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
