// Senkron agent örneği.
// Mesela hava durumu, stok takibi, saat sorgusu gibi gibi..

package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

// slack_send_message'in /run_task'a gönderdiği "arguments" JSON'una karşılık gelen struct
type SendMessageArgs struct {
	ChannelID string `json:"channel_id"`
	Text      string `json:"text"`
}

// slack_read_messages için "arguments" struct'ı
type ReadMessagesArgs struct {
	ChannelID string `json:"channel_id"`
	Limit     int    `json:"limit"`
}

// Sahte 'send_message' handler'ı
func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	log.Println("[Fake Slack Agent] /send_message çağrıldı.")

	// Orchestrator'dan gelen JSON argümanlarını oku
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Request body okunamadı", http.StatusBadRequest)
		return
	}

	var args SendMessageArgs
	if err := json.Unmarshal(body, &args); err != nil {
		http.Error(w, "Geçersiz JSON formatı", http.StatusBadRequest)
		return
	}

	// (Burada normalde gerçek Slack API'ına istek atılır)
	log.Printf("[Fake Slack Agent] İşlem BAŞARILI: Kanal %s, Mesaj: %s", args.ChannelID, args.Text)

	// Orchestrator'a başarılı cevabı dön
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"status":    "sahte mesaj gönderildi",
		"timestamp": "123456789.001",
	})
}

// Sahte 'read_messages' handler'ı
func handleReadMessages(w http.ResponseWriter, r *http.Request) {
	log.Println("[Fake Slack Agent] /read_messages çağrıldı.")

	var args ReadMessagesArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, "Geçersiz JSON formatı", http.StatusBadRequest)
		return
	}

	// (Burada normalde gerçek Slack API'ından mesajlar çekilir)
	log.Printf("[Fake Slack Agent] İşlem BAŞARILI: Kanal %s, Limit: %d", args.ChannelID, args.Limit)

	// Orchestrator'a sahte mesaj listesi dön
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
		"messages": []map[string]string{
			{"user": "U123", "text": "İlk sahte mesaj"},
			{"user": "U456", "text": "İkinci sahte mesaj"},
		},
	})
}

func main() {
	// config/agents.json dosyamızdaki endpoint'leri burada tanımlıyoruz
	mux := http.NewServeMux()
	mux.HandleFunc("/send_message", handleSendMessage)
	mux.HandleFunc("/read_messages", handleReadMessages)

	// NOT: config'de "http://slack-service:8081" yazdık.
	// Docker kullanmıyorsak, localhost'ta çalıştırırken
	// config dosyasındaki "slack-service" kısmını "localhost" yapmamız gerekebilir.
	// Şimdilik 8081 portunda başlatalım.
	log.Println("[Fake Slack Agent] Sahte Slack servisi http://localhost:8081 adresinde başlatılıyor...")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Fatalf("Sahte agent başlatılamadı: %v", err)
	}
}
