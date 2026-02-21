package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/xeipuuv/gojsonschema"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// CalendarAgent yapısı
type CalendarAgent struct {
	calSrv      *calendar.Service
	tasksMu     sync.RWMutex
	tasks       map[string]map[string]interface{}
	cancelFuncs map[string]context.CancelFunc

	requestSchema  *gojsonschema.Schema
	responseSchema *gojsonschema.Schema
}

func loadSchemas() (*gojsonschema.Schema, *gojsonschema.Schema) {
	cwd, _ := os.Getwd()
	rawPath := filepath.Join(cwd, "..", "..", "schemas", "task_schema.json")
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		log.Fatalf("❌ Dosya yolu çözümlenemedi: %v", err)
	}

	absPath = filepath.ToSlash(absPath)
	if !strings.HasPrefix(absPath, "/") {
		absPath = "/" + absPath
	}
	schemaURI := "file://" + absPath

	log.Printf("📂 Şema Yükleniyor: %s", schemaURI)

	reqLoader := gojsonschema.NewReferenceLoader(schemaURI + "#/$defs/OrchestratorTaskRequest")
	reqSchema, err := gojsonschema.NewSchema(reqLoader)
	if err != nil {
		log.Fatalf("❌ Request Schema yüklenemedi: %v", err)
	}

	resLoader := gojsonschema.NewReferenceLoader(schemaURI + "#/$defs/TaskStatusResponse")
	resSchema, err := gojsonschema.NewSchema(resLoader)
	if err != nil {
		log.Fatalf("❌ Response Schema yüklenemedi: %v", err)
	}

	return reqSchema, resSchema
}

func initCalendarService() *calendar.Service {
	ctx := context.Background()
	_ = godotenv.Load("../../.env")

	b, err := os.ReadFile("../../secrets/calendar_api.json")
	if err != nil {
		log.Fatalf("❌ Secret okunamadı: %v", err)
	}

	config, err := google.JWTConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("❌ JWT hatası: %v", err)
	}
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(config.Client(ctx)))
	if err != nil {
		log.Fatalf("❌ Google Calendar Service hatası: %v", err)
	}
	return srv
}

func main() {
	reqSchema, resSchema := loadSchemas()
	log.Println("✅ task_schema.json başarıyla yüklendi.")

	srv := initCalendarService()

	agent := &CalendarAgent{
		calSrv:         srv,
		tasks:          make(map[string]map[string]interface{}),
		cancelFuncs:    make(map[string]context.CancelFunc),
		requestSchema:  reqSchema,
		responseSchema: resSchema,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/execute", agent.handleExecute)
	mux.HandleFunc("/task_status/", agent.handleStatus)
	mux.HandleFunc("/task_stop/", agent.handleStop)

	log.Println("🚀 Calendar Agent 8082 portunda dinliyor...")
	if err := http.ListenAndServe(":8082", mux); err != nil {
		log.Fatal(err)
	}
}

func (a *CalendarAgent) handleExecute(w http.ResponseWriter, r *http.Request) {
	var bodyMap map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&bodyMap); err != nil {
		log.Printf("❌ JSON Decode Hatası: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 📥 GELEN HAM VERİYİ LOGLA
	rawJSON, _ := json.Marshal(bodyMap)
	log.Printf("📩 Yeni İstek Geldi: %s", string(rawJSON))

	_, hasFunc := bodyMap["function_name"]
	_, hasArgs := bodyMap["arguments"]

	if !hasFunc || !hasArgs {
		log.Println("⚠️  Eksik alanlar tespit edildi, paket simüle ediliyor (wrapping)...")
		newBody := map[string]interface{}{
			"function_name": "create_calendar_event",
			"agent_name":    "google_calendar_agent",
			"arguments":     bodyMap,
		}
		bodyMap = newBody
	}

	// 🔍 ŞEMA DOĞRULAMA
	loader := gojsonschema.NewGoLoader(bodyMap)
	result, err := a.requestSchema.Validate(loader)
	if err != nil {
		log.Printf("❌ Validasyon Hatası: %v", err)
		http.Error(w, "Validation Internal Error", http.StatusInternalServerError)
		return
	}

	if !result.Valid() {
		log.Printf("❌ Şema Doğrulanamadı: %v", result.Errors())
		http.Error(w, "Schema Validation Failed", http.StatusBadRequest)
		return
	}

	// 📍 PARAMETRELERİ AYRIŞTIR VE LOGLA
	args := bodyMap["arguments"].(map[string]interface{})
	summary, _ := args["summary"].(string)
	startTime, _ := args["start_time"].(string)
	endTime, _ := args["end_time"].(string)

	if endTime == "" && startTime != "" {
		// ISO 8601 formatını parse et (time.RFC3339)
		t, err := time.Parse(time.RFC3339, startTime)
		if err == nil {
			endTime = t.Add(time.Hour).Format(time.RFC3339)
			log.Printf("⚠️  Bitiş zamanı boş, otomatik +1 saat eklendi: %s", endTime)
		} else {
			log.Printf("❌ Başlangıç zamanı parse edilemedi: %v", err)
		}
	}

	log.Printf("🧩 Ayrıştırılan Parametreler: Summary='%s', Start='%s', End='%s'", summary, startTime, endTime)

	taskID := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())

	a.tasksMu.Lock()
	a.tasks[taskID] = map[string]interface{}{"task_id": taskID, "status": "pending"}
	a.cancelFuncs[taskID] = cancel
	a.tasksMu.Unlock()

	log.Printf("📝 Görev Kaydedildi: TaskID=%s", taskID)

	go a.runTask(ctx, taskID, summary, startTime, endTime)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"task_id": taskID, "status": "pending"})
}

func (a *CalendarAgent) runTask(ctx context.Context, taskID, summary, start, end string) {
	a.updateStatus(taskID, "running", nil, nil)
	log.Printf("⚙️  Görev Başlatılıyor (TaskID: %s)...", taskID)

	event := &calendar.Event{
		Summary: summary,
		Start:   &calendar.EventDateTime{DateTime: start, TimeZone: "Europe/Istanbul"},
		End:     &calendar.EventDateTime{DateTime: end, TimeZone: "Europe/Istanbul"},
	}

	calendarId := os.Getenv("GMAIL_ADDRESS")
	if calendarId == "" {
		calendarId = "primary"
	}

	log.Printf("📅 Google Calendar'a Gönderiliyor: %s (%s - %s)", summary, start, end)

	createdEvent, err := a.calSrv.Events.Insert(calendarId, event).Context(ctx).Do()

	if err != nil {
		errStr := err.Error()
		log.Printf("❌ API Hatası (TaskID: %s): %v", taskID, errStr)
		a.updateStatus(taskID, "failed", nil, &errStr)
	} else {
		log.Printf("✅ Başarılı! (TaskID: %s). Link: %s", taskID, createdEvent.HtmlLink)
		res := map[string]string{"htmlLink": createdEvent.HtmlLink}
		a.updateStatus(taskID, "completed", res, nil)
	}
}

func (a *CalendarAgent) updateStatus(taskID, status string, result interface{}, errStr *string) {
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

	// Durum değişikliğini logla
	log.Printf("🔄 Durum Güncellendi [%s]: %s", taskID, status)

	a.tasks[taskID] = statusObj
}

// handleStatus ve handleStop kısımları aynı kalabilir...
func (a *CalendarAgent) handleStatus(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/task_status/")
	log.Printf("📡 Durum Sorgusu: %s", taskID)

	a.tasksMu.RLock()
	task, ok := a.tasks[taskID]
	a.tasksMu.RUnlock()

	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (a *CalendarAgent) handleStop(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/task_stop/")
	log.Printf("🛑 Durdurma İsteği: %s", taskID)

	a.tasksMu.Lock()
	cancel, ok := a.cancelFuncs[taskID]
	a.tasksMu.Unlock()

	if ok && cancel != nil {
		cancel()
	}
	w.WriteHeader(http.StatusOK)
}
