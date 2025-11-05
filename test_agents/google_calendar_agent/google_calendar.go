package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
	"github.com/joho/godotenv"
)

// calendarSrv, tüm handler'lar tarafından erişilebilen global bir değişken olacak.
var calSrv *calendar.Service

// Orchestrator'dan gelen "arguments" JSON'una karşılık gelen struct
type CreateEventArgs struct {
	Summary     string `json:"summary"`
	//kafa karıştırıyor Description string `json:"description"`
	StartTime   string `json:"start_time"` // LLM'in RFC3339 formatında (string) göndereceğini varsayıyoruz
	EndTime     string `json:"end_time"`   // Örn: "2025-11-06T14:00:00+03:00"
}

// initCalendarService, sunucu başlarken Google API'a bağlanır.
func initCalendarService() {
	ctx := context.Background()

	// 1. Credentials dosyasını oku
	// (Dosya yolunun, bu Go dosyasının çalıştığı yere göre doğru olduğundan emin ol)
	b, err := os.ReadFile("../../secrets/calendar_api.json")
	if err != nil {
		log.Fatalf("Kimlik bilgisi dosyası okunamadı: %v", err)
	}

	// 2. Google JWT yapılandırmasını ayarla
	config, err := google.JWTConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("JWT yapılandırması oluşturulamadı: %v", err)
	}

	// 3. HTTP client'ı oluştur
	client := config.Client(ctx)

	// 4. Calendar servisini başlat ve global değişkene ata
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Calendar servisi başlatılamadı: %v", err)
	}
	
	calSrv = srv // Global değişkene atıyoruz
	log.Println("Google Calendar servisi başarıyla bağlandı.")
}

// handleCreateEvent, /create_event endpoint'ine gelen istekleri karşılar
func handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	log.Println("[Calendar Agent] /create_event çağrıldı.")

	// 1. Orchestrator'dan gelen JSON argümanlarını oku
	var args CreateEventArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		log.Printf("Hata: Geçersiz JSON formatı: %v", err)
		http.Error(w, "Geçersiz JSON formatı", http.StatusBadRequest)
		return
	}
	
	// 2. LLM'den gelen bilgilere göre Event objesi oluştur
	event := &calendar.Event{
		Summary:     args.Summary,
		Start: &calendar.EventDateTime{
			DateTime: args.StartTime, // LLM'den gelen RFC3339 string'i
			TimeZone: "Europe/Istanbul", // Veya Timezone'u da argüman olarak alabilirsiniz
		},
		End: &calendar.EventDateTime{
			DateTime: args.EndTime, // LLM'den gelen RFC3339 string'i
			TimeZone: "Europe/Istanbul",
		},
	}

	// 3. Etkinliği takvime ekle
	calendarId := os.Getenv("GMAIL_ADDRESS")
    if calendarId == "" {
    	log.Println("UYARI: GMAIL_ADDRESS .env dosyasında ayarlanmamış. 'primary' kullanılacak.")
    	calendarId = "primary"
    }
	createdEvent, err := calSrv.Events.Insert(calendarId, event).Do()
	if err != nil {
		log.Printf("Hata: Etkinlik oluşturulamadı: %v", err)
		http.Error(w, "Etkinlik oluşturulamadı", http.StatusInternalServerError)
		return
	}

	log.Printf("İşlem BAŞARILI: Etkinlik oluşturuldu: %s", createdEvent.HtmlLink)

	// 4. Orchestrator'a başarılı cevabı dön
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":           true,
		"status":       "etkinlik oluşturuldu",
		"summary":      createdEvent.Summary,
		"htmlLink":     createdEvent.HtmlLink,
	})
}

// main fonksiyonu artık sadece HTTP sunucusunu başlatır
func main() {
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Println("Uyarı: .env dosyası bulunamadı.")
	}

	// Önce Google Calendar'a bağlan
	initCalendarService()

	// config/agents.json dosyamızdaki endpoint'i burada tanımlıyoruz
	mux := http.NewServeMux()
	mux.HandleFunc("/create_event", handleCreateEvent)

	// Agent'ı 8082 portunda başlat
	log.Println("[Calendar Agent] Google Calendar agent servisi http://localhost:8082 adresinde başlatılıyor...")
	if err := http.ListenAndServe(":8082", mux); err != nil {
		log.Fatalf("Calendar agent başlatılamadı: %v", err)
	}
}