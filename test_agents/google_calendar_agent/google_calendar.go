package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
		log.Fatalf("Dosya yolu çözümlenemedi: %v", err)
	}

	absPath = filepath.ToSlash(absPath)

	if !strings.HasPrefix(absPath, "/") {
		absPath = "/" + absPath
	}

	schemaURI := "file://" + absPath

	log.Printf("📂 Yüklenen Şema Yolu: %s", schemaURI)

	reqLoader := gojsonschema.NewReferenceLoader(schemaURI + "#/$defs/OrchestratorTaskRequest")
	reqSchema, err := gojsonschema.NewSchema(reqLoader)
	if err != nil {
		log.Fatalf("Request Schema yüklenemedi: %v", err)
	}

	resLoader := gojsonschema.NewReferenceLoader(schemaURI + "#/$defs/TaskStatusResponse")
	resSchema, err := gojsonschema.NewSchema(resLoader)
	if err != nil {
		log.Fatalf("Response Schema yüklenemedi: %v", err)
	}

	return reqSchema, resSchema
}

func initCalendarService() *calendar.Service {
	ctx := context.Background()
	_ = godotenv.Load("../../.env")

	b, err := os.ReadFile("../../secrets/calendar_api.json")
	if err != nil {
		log.Fatalf("Secret okunamadı (secrets/calendar_api.json): %v", err)
	}

	config, err := google.JWTConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("JWT hatası: %v", err)
	}
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(config.Client(ctx)))
	if err != nil {
		log.Fatalf("Service hatası: %v", err)
	}
	return srv
}

func main() {
	reqSchema, resSchema := loadSchemas()
	log.Println("✅ task_schema.json başarıyla yüklendi ve parse edildi.")

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

	log.Println("🚀 Calendar Agent (Dynamic Schema) 8082 portunda çalışıyor...")
	if err := http.ListenAndServe(":8082", mux); err != nil {
		log.Fatal(err)
	}
}

func (a *CalendarAgent) handleExecute(w http.ResponseWriter, r *http.Request) {
	var bodyMap map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&bodyMap); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if _, ok := bodyMap["agent_name"]; !ok {
		// Gelen veriyi "arguments" içine taşıyoruz
		newBody := map[string]interface{}{
			"agent_name": "create_calendar_event", // Adını biz koyalım
			"arguments":  bodyMap,                 // Gelen her şeyi argüman say
		}
		bodyMap = newBody
	}

	loader := gojsonschema.NewGoLoader(bodyMap)
	result, err := a.requestSchema.Validate(loader)
	if err != nil {
		http.Error(w, "Validation Internal Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !result.Valid() {
		var sb strings.Builder
		for _, desc := range result.Errors() {
			sb.WriteString(fmt.Sprintf("- %s\n", desc))
		}
		http.Error(w, "Schema Validation Failed:\n"+sb.String(), http.StatusBadRequest)
		return
	}

	args := bodyMap["arguments"].(map[string]interface{})

	summary, _ := args["summary"].(string)
	startTime, _ := args["start_time"].(string)
	endTime, _ := args["end_time"].(string)

	taskID := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())

	initialState := map[string]interface{}{
		"task_id": taskID,
		"status":  "pending",
	}

	a.tasksMu.Lock()
	a.tasks[taskID] = initialState
	a.cancelFuncs[taskID] = cancel
	a.tasksMu.Unlock()

	go a.runTask(ctx, taskID, summary, startTime, endTime)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id": taskID,
		"status":  "pending",
	})
}

func (a *CalendarAgent) runTask(ctx context.Context, taskID, summary, start, end string) {
	a.updateStatus(taskID, "running", nil, nil)

	event := &calendar.Event{
		Summary: summary,
		Start:   &calendar.EventDateTime{DateTime: start, TimeZone: "Europe/Istanbul"},
		End:     &calendar.EventDateTime{DateTime: end, TimeZone: "Europe/Istanbul"},
	}

	calendarId := os.Getenv("GMAIL_ADDRESS")
	if calendarId == "" {
		calendarId = "primary"
	}

	createdEvent, err := a.calSrv.Events.Insert(calendarId, event).Context(ctx).Do()

	if err != nil {
		errStr := err.Error()
		if err == context.Canceled {
			errStr = "Task canceled"
		}
		a.updateStatus(taskID, "failed", nil, &errStr)
	} else {
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

	loader := gojsonschema.NewGoLoader(statusObj)
	res, _ := a.responseSchema.Validate(loader)
	if !res.Valid() {
		log.Printf("⚠️  DIKKAT: Agent hatalı JSON üretiyor (Task: %s): %v", taskID, res.Errors())
	}

	a.tasks[taskID] = statusObj
}

func (a *CalendarAgent) handleStatus(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/task_status/")

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

	a.tasksMu.Lock()
	cancel, ok := a.cancelFuncs[taskID]
	a.tasksMu.Unlock()

	if ok && cancel != nil {
		cancel()
	}
	w.WriteHeader(http.StatusOK)
}
