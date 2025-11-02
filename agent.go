package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// AgentDefinition, config/agents.json dosyas�ndaki her bir objeye kar��l�k gelir.
type AgentDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
	Endpoint    string          `json:"endpoint"` // Art�k JSON'dan okunuyor
}

// AgentRegistry (hi� de�i�medi)
type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]AgentDefinition
}

// NewAgentRegistry (hi� de�i�medi)
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]AgentDefinition),
	}
}

// Register (hi� de�i�medi)
func (r *AgentRegistry) Register(def AgentDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[def.Name] = def
}

// Get (hi� de�i�medi)
func (r *AgentRegistry) Get(name string) (AgentDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[name]
	return agent, ok
}

// GetToolsSpec (Endpoint bilgisini gizleyecek �ekilde g�ncellendi)
func (r *AgentRegistry) GetToolsSpec() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// LLM'e sadece name, description ve schema'y� g�ndeririz.
	// Endpoint bir i� uygulama detay�d�r.
	specs := make([]map[string]interface{}, 0, len(r.agents))
	for _, agent := range r.agents {
		specs = append(specs, map[string]interface{}{
			"name":        agent.Name,
			"description": agent.Description,
			"schema":      agent.Schema,
		})
	}
	return specs
}

// YEN� FONKS�YON: LoadAgentsFromConfig
// config/agents.json dosyas�n� okur ve t�m agent'lar� registry'ye kaydeder.
func LoadAgentsFromConfig(registry *AgentRegistry, configFile string) error {
	log.Printf("Agent konfig�rasyonu y�kleniyor: %s", configFile)

	file, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var definitions []AgentDefinition
	if err := json.NewDecoder(file).Decode(&definitions); err != nil {
		return err
	}

	count := 0
	for _, def := range definitions {
		registry.Register(def)
		count++
	}

	log.Printf("%d agent eylemi ba�ar�yla y�klendi.", count)
	return nil
}
