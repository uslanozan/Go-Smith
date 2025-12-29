package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xeipuuv/gojsonschema"
)

// SlackAgent Yapısı
type SlackAgent struct {
	tasksMu sync.RWMutex
	// Veritabanı yerine geçen in-memory map
	tasks map[string]map[string]interface{}

	// Şemalar
	requestSchema  *gojsonschema.Schema
	responseSchema *gojsonschema.Schema
}

// Şemaları Yükle (Windows Uyumlu)
func loadSchemas() (*gojsonschema.Schema, *gojsonschema.Schema) {
	cwd, _ := os.Getwd()
	// Proje yapına göre path'i ayarla (test_agents/fake_slack_agent olduğunu varsayıyoruz)
	rawPath := filepath.Join(cwd, "..", "..", "schemas", "task_schema.json")
	
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		log.Fatalf("Dosya yolu hatası: %v", err)
	}

	// Windows Path Düzeltmesi (C:\ -> /C:/ ve \)
	absPath = filepath.ToSlash(absPath)
	if !strings.HasPrefix(absPath, "/") {
		absPath = "/" + absPath
	}
	schemaURI := "file://" + absPath

	log.Printf("📂 Şema Yolu: %s", schemaURI)

	// Request Validator
	reqLoader := gojsonschema.NewReferenceLoader(schemaURI + "#/$defs/OrchestratorTaskRequest")
	reqSchema, err := gojsonschema.NewSchema(reqLoader)
	if err != nil {
		log.Fatalf("Request Schema yüklenemedi: %v", err)
	}

	// Response Validator
	resLoader := gojsonschema.NewReferenceLoader(schemaURI + "#/$defs/TaskStatusResponse")
	resSchema, err := gojsonschema.NewSchema(resLoader)
	if err != nil {
		log.Fatalf("Response Schema yüklenemedi: %v", err)
	}

	return reqSchema, resSchema
}

func main() {
	reqSchema, resSchema := loadSchemas()
	log.Println("✅ task_schema.json yüklendi.")

	agent := &SlackAgent{
		tasks:          make(map[string]map[string]interface{}),
		requestSchema:  reqSchema,
		responseSchema: resSchema,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/send_message", agent.handleSendMessage)
	mux.HandleFunc("/read_messages", agent.handleReadMessages)
	mux.HandleFunc("/task_status/", agent.handleStatus) // Ortak durum sorgulama

	log.Println("[Fake Slack Agent] Schema-Based servis http://localhost:8081 adresinde çalışıyor...")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Fatalf("Başlatılamadı: %v", err)
	}
}

// --------------------------------------------------------------------------------
// HANDLER: SEND MESSAGE
// --------------------------------------------------------------------------------
func (a *SlackAgent) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	// 1. Validasyon ve Parse (Ortak Helper Kullanılabilir ama açık yazıyorum)
	bodyMap, err := a.validateRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 2. Argümanları Al
	args := bodyMap["arguments"].(map[string]interface{})
	channelID, _ := args["channel_id"].(string)
	text, _ := args["text"].(string)

	// 3. Task Oluştur
	taskID := uuid.NewString()

	log.Printf("[Slack] Mesaj Gönderiliyor -> Kanal: %s, Mesaj: %s", channelID, text)

	// 4. İŞLEMİ YAP (Fake olduğu için anında yapıp sonucu kaydediyoruz)
	// Senkron bir iş olsa bile "Asenkron Protokol"e uyduruyoruz.
	resultData := map[string]interface{}{
		"ok":        true,
		"status":    "mesaj iletildi",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
		"channel":   channelID,
	}

	// Durumu direkt "Completed" olarak kaydediyoruz
	a.saveTaskState(taskID, "completed", resultData, nil)

	// 5. Cevap Dön (TaskStartResponse)
	// Orchestrator bu cevabı alınca "Tamam iş başladı" diyecek ve hemen ardından /task_status soracak.
	// Sorduğunda da yukarıda kaydettiğimiz "completed" sonucunu alacak.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id": taskID,
		"status":  "pending", // Protokol gereği "başlattım" diyoruz.
	})
}

// --------------------------------------------------------------------------------
// HANDLER: READ MESSAGES
// --------------------------------------------------------------------------------
func (a *SlackAgent) handleReadMessages(w http.ResponseWriter, r *http.Request) {
	bodyMap, err := a.validateRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	args := bodyMap["arguments"].(map[string]interface{})
	channelID, _ := args["channel_id"].(string)
	// JSON'dan sayılar float64 olarak gelir
	limitFloat, _ := args["limit"].(float64)
	limit := int(limitFloat)

	taskID := uuid.NewString()
	log.Printf("[Slack] Mesajlar Okunuyor -> Kanal: %s, Limit: %d", channelID, limit)

	// Fake Sonuç
	fakeMessages := []map[string]string{
		{"user": "ozan", "text": "Selamlar"},
		{"user": "bot", "text": "Task tamamlandı"},
	}
	
	// Limite göre kes (fake logic)
	if limit > 0 && limit < len(fakeMessages) {
		fakeMessages = fakeMessages[:limit]
	}

	resultData := map[string]interface{}{
		"ok":       true,
		"messages": fakeMessages,
		"count":    len(fakeMessages),
	}

	// Anında bitti olarak kaydet
	a.saveTaskState(taskID, "completed", resultData, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id": taskID,
		"status":  "pending",
	})
}

// --------------------------------------------------------------------------------
// ORTAK YARDIMCILAR
// --------------------------------------------------------------------------------

func (a *SlackAgent) handleStatus(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/task_status/")
	
	a.tasksMu.RLock()
	task, ok := a.tasks[taskID]
	a.tasksMu.RUnlock()

	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// İsteği okur, JSON map'e çevirir ve Şema ile doğrular
func (a *SlackAgent) validateRequest(r *http.Request) (map[string]interface{}, error) {
	var bodyMap map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&bodyMap); err != nil {
		return nil, fmt.Errorf("Invalid JSON")
	}

	loader := gojsonschema.NewGoLoader(bodyMap)
	result, err := a.requestSchema.Validate(loader)
	if err != nil {
		return nil, fmt.Errorf("Validation Internal Error: %v", err)
	}

	if !result.Valid() {
		var sb strings.Builder
		sb.WriteString("Schema Validation Failed: ")
		for _, desc := range result.Errors() {
			sb.WriteString(fmt.Sprintf("[%s] ", desc))
		}
		return nil, fmt.Errorf("%s", sb.String())
	}

	return bodyMap, nil
}

// Durumu kaydeder ve Response Schema ile doğruluğunu (log için) kontrol eder
func (a *SlackAgent) saveTaskState(taskID, status string, result interface{}, errStr *string) {
	a.tasksMu.Lock()
	defer a.tasksMu.Unlock()

	statusObj := map[string]interface{}{
		"task_id": taskID,
		"status":  status,
	}
	if result != nil {
		statusObj["result"] = result
	}
	if errStr != nil {
		statusObj["error"] = *errStr
	}

	// Response Schema Check (Debug amaçlı)
	loader := gojsonschema.NewGoLoader(statusObj)
	res, _ := a.responseSchema.Validate(loader)
	if !res.Valid() {
		log.Printf("⚠️ INTERNAL WARNING: Response schema hatası (Task: %s): %v", taskID, res.Errors())
	}

	a.tasks[taskID] = statusObj
}