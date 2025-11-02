package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

// TaskRequest, LLM'den Orchestrator'a gelecek olan JSON isteğinin formatıdır.
type TaskRequest struct {
	AgentName string `json:"agent_name"`
	// Argümanları ham JSON (RawMessage) olarak alıyoruz,
	// böylece değiştirmeden doğrudan agent'a iletebiliriz.
	Arguments json.RawMessage `json:"arguments"`
}

// Orchestrator, ana sunucu yapımız.
// Agent kayıt defterini ve diğer servislere istek atmak için bir HTTP client'ı tutar.
type Orchestrator struct {
	Registry   *AgentRegistry
	HttpClient *http.Client
}

func NewOrchestrator(registry *AgentRegistry) *Orchestrator {
	return &Orchestrator{
		Registry: registry,
		HttpClient: &http.Client{
			Timeout: 10 * time.Second, // Agent'lara istek atarken zaman aşımı
		},
	}
}

// HandleTask, bizim ana dağıtıcı (dispatch) handler'ımız.
// LLM'den gelen "/run_task" gibi bir isteği bu fonksiyon karşılar.
func (o *Orchestrator) HandleTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. LLM'den gelen JSON isteğini parse et
	var task TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 2. Agent'ı kayıt defterinde bul
	agent, ok := o.Registry.Get(task.AgentName)
	if !ok {
		log.Printf("Hata: Bilinmeyen agent istendi: %s", task.AgentName)
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	log.Printf("Görev alındı: Agent '%s', Endpoint: '%s'", agent.Name, agent.Endpoint)

	// 3. Görevi (argümanları) ilgili agent'a PUSH et
	// Argümanları (task.Arguments) bir buffer'a koyup POST isteği olarak gönderiyoruz.
	agentReq, err := http.NewRequest("POST", agent.Endpoint, bytes.NewBuffer(task.Arguments))
	if err != nil {
		log.Printf("Hata: Agent isteği oluşturulamadı: %v", err)
		http.Error(w, "Failed to create agent request", http.StatusInternalServerError)
		return
	}
	agentReq.Header.Set("Content-Type", "application/json")

	// 4. Agent servisinden gelen cevabı al
	agentResp, err := o.HttpClient.Do(agentReq)
	if err != nil {
		// Agent servisi çalışmıyor veya hata verdi
		log.Printf("Hata: Agent '%s' çağrılamadı: %v", agent.Name, err)
		http.Error(w, "Failed to call agent service", http.StatusServiceUnavailable)
		return
	}
	defer agentResp.Body.Close()

	// 5. Agent'ın cevabını (başarılı veya hatalı) doğrudan bizi çağıran servise (LLM'e) geri yolla.
	// Bu, Orchestrator'ı bir "proxy" (vekil) gibi davranmasını sağlar.
	log.Printf("Agent '%s' yanıt verdi: %s", agent.Name, agentResp.Status)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(agentResp.StatusCode)
	io.Copy(w, agentResp.Body)
}

// HandleGetTools, LLM'e "hangi araçların var?" diye sorması için bir endpoint sağlar.
func (o *Orchestrator) HandleGetTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	tools := o.Registry.GetToolsSpec()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tools)
}
