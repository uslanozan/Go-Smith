package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/uslanozan/Go-Smith/models"
)
type TaskState struct {
	Response   models.TaskStatusResponse
	CancelFunc context.CancelFunc
}

type PdfArgs struct {
    FileName string `json:"file_name"`
}

type PdfAgent struct {
    tasksMu sync.RWMutex
    tasks   map[string]TaskState
}
// ------------------------------------------

func main() {
    agent := &PdfAgent{
        tasks: make(map[string]TaskState),
    }

    mux := http.NewServeMux()
    
    mux.HandleFunc("/convert_pdf", agent.handleConvertPdf) 
    mux.HandleFunc("/task_status/", agent.handleTaskStatus)
    mux.HandleFunc("/task_stop/", agent.handleStopTask)

    log.Println("[PDF Agent] Asenkron PDF agent servisi http://localhost:8083 adresinde başlatılıyor...")
    if err := http.ListenAndServe(":8083", mux); err != nil {
        log.Fatalf("PDF agent başlatılamadı: %v", err)
    }
}

func (a *PdfAgent) handleConvertPdf(w http.ResponseWriter, r *http.Request) {
    // HER İSTEK İÇİN YENİ BİR FORM OLUŞTURULUR
    var args PdfArgs 
    if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
        // ...
    }
    // ...
}

func (a *PdfAgent) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/task_status/")
	if taskID == "" {
		http.Error(w, "Task ID eksik", http.StatusBadRequest)
		return
	}
	log.Printf("[PDF Agent] /task_status/%s çağrıldı.", taskID)

	a.tasksMu.RLock()
	state, ok := a.tasks[taskID]
	a.tasksMu.RUnlock()

	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state.Response)
}

func (a *PdfAgent) handleStopTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	
	taskID := strings.TrimPrefix(r.URL.Path, "/task_stop/")
	log.Printf("[PDF Agent] /task_stop/%s çağrıldı.", taskID)

	a.tasksMu.Lock() // Yazma kilidi (garanti olsun)
	state, ok := a.tasks[taskID]
	a.tasksMu.Unlock()

	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// SİHİRLİ AN: Kaydettiğimiz cancel fonksiyonunu çağırıyoruz!
	// Bu, runCalendarTask içindeki context'i anında öldürür.
	if state.CancelFunc != nil {
		state.CancelFunc()
		log.Printf("Task %s için iptal sinyali tetiklendi.", taskID)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"stop signal sent"}`))
}