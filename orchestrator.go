// Server Side
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/uslanozan/Go-Smith/models"
)

// Orchestrator registry ve diğer servislere istek atmak için bir HTTP client'ı tutar.
type Orchestrator struct {
	Registry   *AgentRegistry
	TaskRegistry *TaskRegistry
	HttpClient *http.Client
}

// Constructor
func NewOrchestrator(registry *AgentRegistry, taskRegistry *TaskRegistry) *Orchestrator {
	return &Orchestrator{
		Registry: registry,
		TaskRegistry: taskRegistry,
		HttpClient: &http.Client{
			Timeout: 10 * time.Second, // Agent'lara istek atarken timeout
		},
	}
}

// LLM'den gelen task'i agent'lara yönlendirir
func (o *Orchestrator) HandleTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	if r.Method != "POST" {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var task models.OrchestratorTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	agent, ok := o.Registry.Get(task.AgentName)
	if !ok {
		log.Printf("Hata: Bilinmeyen agent istendi: %s", task.AgentName)
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	log.Printf("Görev alındı: Agent '%s', Endpoint: '%s'", agent.Name, agent.Endpoint)
	agentReq, err := http.NewRequestWithContext(ctx, "POST", agent.Endpoint, bytes.NewBuffer(task.Arguments))
	agentReq.Header.Set("Content-Type", "application/json")
	
	agentResp, err := o.HttpClient.Do(agentReq)
	if err != nil {
		log.Printf("Hata: Agent '%s' çağrılamadı: %v", agent.Name, err)
		http.Error(w, "Failed to call agent service", http.StatusServiceUnavailable)
		return
	}
	defer agentResp.Body.Close()

	// --- YENİ AKILLI MANTIK ---
	
	// Agent'ın döndüğü HTTP durum kodunu kontrol et
	if agentResp.StatusCode == http.StatusAccepted {
		// DURUM 1: ASENKRON GÖREV (Agent "202 Kabul Edildi" dedi)
		
		// Agent'tan gelen "Sipariş Fişi"ni (TaskStartResponse) oku
		var startResp models.TaskStartResponse
		if err := json.NewDecoder(agentResp.Body).Decode(&startResp); err != nil {
			log.Printf("Hata: Agent'ın asenkron cevabı anlaşılamadı: %v", err)
			http.Error(w, "Agent response parsing error", http.StatusInternalServerError)
			return
		}

		// Görevi (Task ID ve Agent) "Görev Defteri"ne kaydet
		if err := o.TaskRegistry.RegisterTask(startResp.TaskID, agent); err != nil {
			log.Printf("Hata: TaskRegistry'ye kayıt yapılamadı: %v", err)
			http.Error(w, "Task registration error", http.StatusInternalServerError)
			return
		}

		// "Sipariş Fişi"ni (TaskStartResponse) Gateway'e geri yolla
		log.Printf("Agent '%s' görevi kabul etti, TaskID: %s", agent.Name, startResp.TaskID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted) // 202'yi Gateway'e de yansıt
		json.NewEncoder(w).Encode(startResp)

	} else {
		// DURUM 2: SENKRON GÖREV (Slack Agent gibi, 200 OK veya Hata döndü)
		
		// Her zamanki gibi, cevabı olduğu gibi Gateway'e geri yolla (proxy)
		log.Printf("Agent '%s' senkron yanıt verdi: %s", agent.Name, agentResp.Status)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(agentResp.StatusCode)
		io.Copy(w, agentResp.Body)
	}
}

func (o *Orchestrator) HandleTaskStatus(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()


	if r.Method != "GET" {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. URL'den Task ID'yi ayıkla (örn: "/task_status/abc-123" -> "abc-123")
	taskID := strings.TrimPrefix(r.URL.Path, "/task_status/")
	if taskID == "" {
		http.Error(w, "Task ID eksik", http.StatusBadRequest)
		return
	}
	log.Printf("Durum sorgusu alındı: TaskID: %s", taskID)

	// 2. "Görev Defteri"ne bakarak bu task'in bilgilerini bul
	taskInfo, ok := o.TaskRegistry.GetTaskInfo(taskID)
	if !ok {
		log.Printf("Hata: Bilinmeyen TaskID: %s", taskID)
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// 3. Agent'ın durum sorgulama adresini oluştur
	// (örn: "http://localhost:8082/task_status/" + "abc-123")
	fullStatusURL := taskInfo.AgentStatusBaseURL + taskID

	agentReq, err := http.NewRequestWithContext(ctx, "GET", fullStatusURL, nil)
	if err != nil {
		log.Printf("Hata: Agent durum sorgu isteği oluşturulamadı: %v", err)
		http.Error(w, "Request creation failed", http.StatusInternalServerError)
		return
	}

	// 4. İsteği ilgili Agent'a yönlendir (Proxy)
	agentResp, err := o.HttpClient.Do(agentReq)
	if err != nil {
		log.Printf("Hata: Agent '%s' durum sorgulanamadı: %v", taskInfo.AgentName, err)
		http.Error(w, "Agent status check failed", http.StatusServiceUnavailable)
		return
	}
	defer agentResp.Body.Close()

	// 5. Agent'ın durum raporunu (TaskStatusResponse) Gateway'e geri yolla
	log.Printf("Agent '%s' durum yanıtı verdi: %s", taskInfo.AgentName, agentResp.Status)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(agentResp.StatusCode)
	io.Copy(w, agentResp.Body)
}

// GetToolsSpec'i çağırır ve LLM'in araçları görmesini sağlar
func (o *Orchestrator) HandleGetTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	tools := o.Registry.GetToolsSpec()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tools)
}

// HandleTaskStop, durdurma isteğini ilgili agent'a yönlendirir.
func (o *Orchestrator) HandleTaskStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/task_stop/")
	
	// Task bilgisini al
	taskInfo, ok := o.TaskRegistry.GetTaskInfo(taskID)
	if !ok {
		http.Error(w, "Task not found in registry", http.StatusNotFound)
		return
	}

	// ARTIK STRING REPLACE YOK! TEMİZ KOD:
	// taskInfo.AgentStopBaseURL zaten "http://localhost:8082/task_stop/" şeklindedir.
	fullStopURL := taskInfo.AgentStopBaseURL + taskID

	// İsteği yönlendir
	agentReq, _ := http.NewRequest("POST", fullStopURL, nil)
	agentResp, err := o.HttpClient.Do(agentReq)
	if err != nil {
		http.Error(w, "Failed to reach agent", http.StatusServiceUnavailable)
		return
	}
	defer agentResp.Body.Close()

	w.WriteHeader(agentResp.StatusCode)
	io.Copy(w, agentResp.Body)
}
