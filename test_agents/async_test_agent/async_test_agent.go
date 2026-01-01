package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

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

func main() {
	agent := &PdfAgent{
		tasks: make(map[string]TaskState),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/execute", agent.handleExecute)
	mux.HandleFunc("/task_status/", agent.handleTaskStatus)
	mux.HandleFunc("/task_stop/", agent.handleStopTask)

	log.Println("[PDF Agent] Asenkron PDF agent servisi http://localhost:8083 adresinde başlatılıyor...")
	if err := http.ListenAndServe(":8083", mux); err != nil {
		log.Fatalf("PDF agent başlatılamadı: %v", err)
	}
}

func (a *PdfAgent) handleExecute(w http.ResponseWriter, r *http.Request) {
	var args PdfArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	taskID := fmt.Sprintf("go-task-%d", time.Now().Unix()%100000)
	log.Printf("[PDF Agent] Yeni görev alındı: %s (Dosya: %s)", taskID, args.FileName)

	ctx, cancel := context.WithCancel(context.Background())

	initialResult, _ := json.Marshal("PDF conversion started...")

	a.tasksMu.Lock()
	a.tasks[taskID] = TaskState{
		Response: models.TaskStatusResponse{
			TaskID: taskID,
			Status: models.StatusRunning,
			Result: initialResult,
		},
		CancelFunc: cancel,
	}
	a.tasksMu.Unlock()

	go func(tID string, ctx context.Context) {
		select {
		case <-time.After(10 * time.Second):

			a.tasksMu.Lock()
			if state, exists := a.tasks[tID]; exists {
				state.Response.Status = models.StatusCompleted

				finalUrl := fmt.Sprintf("https://cdn.gosmith.local/%s.pdf", args.FileName)
				resultJSON, _ := json.Marshal(map[string]string{
					"download_url": finalUrl,
					"message":      "Conversion successful",
				})
				state.Response.Result = resultJSON

				a.tasks[tID] = state
			}
			a.tasksMu.Unlock()
			log.Printf("[PDF Agent] Görev %s tamamlandı.", tID)

		case <-ctx.Done():
			a.tasksMu.Lock()
			if state, exists := a.tasks[tID]; exists {
				state.Response.Status = models.StatusFailed
				state.Response.Error = "Operation stopped by user request."
				a.tasks[tID] = state
			}
			a.tasksMu.Unlock()
			log.Printf("[PDF Agent] Görev %s durduruldu.", tID)
		}
	}(taskID, ctx)

	startResp := models.TaskStartResponse{
		TaskID: taskID,
		Status: models.StatusRunning,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(startResp)
}

func (a *PdfAgent) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/task_status/")
	if taskID == "" {
		http.Error(w, "Task ID eksik", http.StatusBadRequest)
		return
	}

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

	a.tasksMu.Lock()
	state, ok := a.tasks[taskID]
	a.tasksMu.Unlock()

	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if state.CancelFunc != nil {
		state.CancelFunc()
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"stop signal sent"}`))
}
