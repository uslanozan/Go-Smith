package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/uslanozan/Ollama-the-Agent/models"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type TaskState struct {
	Response   models.TaskStatusResponse
	CancelFunc context.CancelFunc
}

type CreateEventArgs struct {
	Summary   string `json:"summary"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// CalendarAgent, bu servisin "beynidir".
type CalendarAgent struct {
	calSrv *calendar.Service

	// Agent'ın "görev defteri" (in-memory veritabanı)
	tasksMu sync.RWMutex
	tasks   map[string]TaskState
}

// initCalendarService, Google API'a bağlanır.
func initCalendarService() *calendar.Service {
	ctx := context.Background()

	// Sizin .env yolunuzu koruyoruz
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("Uyarı: .env dosyası bulunamadı.")
	}

	// Sizin secrets yolunuzu koruyoruz
	b, err := os.ReadFile("../../secrets/calendar_api.json")
	if err != nil {
		log.Fatalf("Kimlik bilgisi dosyası okunamadı: %v", err)
	}

	config, err := google.JWTConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("JWT yapılandırması oluşturulamadı: %v", err)
	}
	client := config.Client(ctx)
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Calendar servisi başlatılamadı: %v", err)
	}

	log.Println("Google Calendar servisi başarıyla bağlandı.")
	return srv
}

// main fonksiyonu agent'ı "kurar" (setup)
func main() {
	calendarService := initCalendarService()

	agent := &CalendarAgent{
		calSrv: calendarService,
		tasks:  make(map[string]TaskState),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/create_event", agent.handleCreateEvent)
	mux.HandleFunc("/task_status/", agent.handleTaskStatus) // <-- YENİ ENDPOINT
	mux.HandleFunc("/task_stop/", agent.handleStopTask)

	log.Println("[Calendar Agent] Asenkron Google Calendar agent servisi http://localhost:8082 adresinde başlatılıyor...")
	if err := http.ListenAndServe(":8082", mux); err != nil {
		log.Fatalf("Calendar agent başlatılamadı: %v", err)
	}
}

// handleCreateEvent (GÖREVİ BAŞLATIR)
func (a *CalendarAgent) handleCreateEvent(w http.ResponseWriter, r *http.Request) {

	log.Println("[Calendar Agent] /create_event (async) çağrıldı.")

	// 1. Argümanları oku (artık 'models' paketinden geliyor)
	var args CreateEventArgs // <-- GÜNCELLENDİ
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, "Geçersiz JSON formatı", http.StatusBadRequest)
		return
	}

	// 2. Yeni bir benzersiz Task ID oluştur
	taskID := uuid.NewString()

	// Bu context, sunucu kapanmadığı sürece veya iptal edilmedikçe yaşar
	bgCtx := context.Background()

	taskCtx, cancel := context.WithCancel(bgCtx)



	// 4. Görevi "görev defterine" (map) kaydet
	a.tasksMu.Lock()
	a.tasks[taskID] = TaskState{
		Response: models.TaskStatusResponse{
			TaskID: taskID,
			Status: models.StatusPending,
		},
		CancelFunc: cancel, // <-- İPTAL FONKSİYONUNU SAKLIYORUZ
	}
	a.tasksMu.Unlock()

	// 5. ASIL İŞİ ARKA PLANDA (goroutine) BAŞLAT
	go a.runCalendarTask(taskCtx, taskID, args)

	// 6. KULLANICIYA ANINDA CEVAP DÖN (Sipariş Fişi)
	log.Printf("Görev alındı, Task ID: %s", taskID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 "Kabul Edildi"
	json.NewEncoder(w).Encode(models.TaskStartResponse{
		TaskID: taskID,
		Status: models.StatusPending,
	})
}

// runCalendarTask (İŞİ ARKA PLANDA YAPAN GOROUTINE)
func (a *CalendarAgent) runCalendarTask(ctx context.Context, taskID string, args CreateEventArgs) { // <-- GÜNCELLENDİ
	// 1. Görevin durumunu "running" olarak güncelle
	a.tasksMu.Lock()
	if state, exists := a.tasks[taskID]; exists {
		state.Response = models.TaskStatusResponse{TaskID: taskID, Status: models.StatusRunning}
		a.tasks[taskID] = state
	}
	a.tasksMu.Unlock()
	log.Printf("Task %s: Durum 'running' olarak güncellendi.", taskID)
    
    log.Println("TEST: Uyku bitti, Google API'ye gidiliyor...")

	// 2. GERÇEK İŞİ YAP (Google API'ı çağırma)
	event := &calendar.Event{
		Summary: args.Summary,
		Start:   &calendar.EventDateTime{DateTime: args.StartTime, TimeZone: "Europe/Istanbul"},
		End:     &calendar.EventDateTime{DateTime: args.EndTime, TimeZone: "Europe/Istanbul"},
	}

	calendarId := os.Getenv("GMAIL_ADDRESS")
	if calendarId == "" {
		calendarId = "primary"
	}

	createdEvent, err := a.calSrv.Events.Insert(calendarId, event).Context(ctx).Do()

	// 3. İŞ BİTTİĞİNDE SONUCU GÜNCELLE
	a.tasksMu.Lock()
	defer a.tasksMu.Unlock()

	updateState := func(resp models.TaskStatusResponse) {
        if state, exists := a.tasks[taskID]; exists {
            state.Response = resp
            a.tasks[taskID] = state
        }
    }

	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			// İptal durumu
			log.Printf("Task %s: Durum 'failed' olarak güncellendi. SEBEP: İSTEK İPTAL EDİLDİ.", taskID)
			updateState(models.TaskStatusResponse{
				TaskID: taskID,
				Status: models.StatusFailed,
				Error:  "Task canceled by user request.",
			})
		} else {
			// Normal hata durumu
			log.Printf("Task %s: Durum 'failed' olarak güncellendi. Hata: %v", taskID, err)
			updateState(models.TaskStatusResponse{
				TaskID: taskID,
				Status: models.StatusFailed,
				Error:  err.Error(),
			})
		}
	} else {
		// Başarı durumu
		log.Printf("Task %s: Durum 'completed' olarak güncellendi. Link: %s", taskID, createdEvent.HtmlLink)
		resultData, _ := json.Marshal(map[string]string{"htmlLink": createdEvent.HtmlLink})
        updateState(models.TaskStatusResponse{
			TaskID: taskID,
			Status: models.StatusCompleted,
			Result: resultData,
		})
	}
}

// handleTaskStatus (GÖREV DURUMUNU SORGULAR)
func (a *CalendarAgent) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/task_status/")
	if taskID == "" {
		http.Error(w, "Task ID eksik", http.StatusBadRequest)
		return
	}
	log.Printf("[Calendar Agent] /task_status/%s çağrıldı.", taskID)

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

func (a *CalendarAgent) handleStopTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	
	taskID := strings.TrimPrefix(r.URL.Path, "/task_stop/")
	log.Printf("[Calendar Agent] /task_stop/%s çağrıldı.", taskID)

	a.tasksMu.Lock() // Yazma kilidi (garanti olsun)
	state, ok := a.tasks[taskID]
	a.tasksMu.Unlock()

	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if state.CancelFunc != nil {
		state.CancelFunc()
		log.Printf("Task %s için iptal sinyali tetiklendi.", taskID)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"stop signal sent"}`))
}