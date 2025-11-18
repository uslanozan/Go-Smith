// Dosya: llm_gateway/gateway.go

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings" // <-- YENİ IMPORT
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/uslanozan/Ollama-the-Agent/models"
)

// GatewayConfig, .env dosyasından yüklenen tüm ayarları tutar.
type GatewayConfig struct {
	OllamaURL              string
	OrchestratorToolsURL   string
	OrchestratorRunTaskURL string
	OllamaModel            string
	HttpClientTimeout      time.Duration
	ListenAddress          string
}

// Gateway, ana sunucu yapımızdır.
type Gateway struct {
	Config     *GatewayConfig
	HttpClient *http.Client
	authKeys   map[string]string
	historyMu  sync.RWMutex
	history    map[string][]models.OllamaMessage
}

// NewGatewayConfig, .env dosyasını okur ve bir GatewayConfig struct'ı oluşturur.
func NewGatewayConfig() (*GatewayConfig, error) {
	timeoutStr := os.Getenv("HTTP_CLIENT_TIMEOUT_SECONDS")
	if timeoutStr == "" {
		timeoutStr = "60"
	}
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("HTTP_CLIENT_TIMEOUT_SECONDS geçersiz: %v", err)
	}

	listenAddr := os.Getenv("GATEWAY_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8000"
	}

	return &GatewayConfig{
		OllamaURL:              os.Getenv("OLLAMA_URL"),
		OrchestratorToolsURL:   os.Getenv("ORCHESTRATOR_TOOLS_URL"),
		OrchestratorRunTaskURL: os.Getenv("ORCHESTRATOR_RUN_TASK_URL"),
		OllamaModel:            os.Getenv("OLLAMA_MODEL"),
		HttpClientTimeout:      time.Duration(timeout) * time.Second,
		ListenAddress:          listenAddr,
	}, nil
}

// NewGateway, yeni bir Gateway servisi oluşturur.
func NewGateway(cfg *GatewayConfig, authKeys map[string]string) *Gateway {
	return &Gateway{
		Config: cfg,
		HttpClient: &http.Client{
			Timeout: cfg.HttpClientTimeout,
		},
		authKeys: authKeys,
		history:  make(map[string][]models.OllamaMessage),
	}
}

// Kullanıcıdan /chat endpoint'ine gelecek format (Gateway'e özel)
type ChatRequest struct {
	Prompt string `json:"prompt"`
}

// --- 1. Adım: Orchestrator'dan Araç Listesini Al ---
func (g *Gateway) getToolsFromOrchestrator(ctx context.Context) ([]models.ToolSpec, error) {

	// GÜNCELLENDİ: Context'i içeren yeni bir GET isteği oluştur
	req, err := http.NewRequestWithContext(ctx, "GET", g.Config.OrchestratorToolsURL, nil)
	if err != nil {
		return nil, err
	}

	// GÜNCELLENDİ: 'req' (context'i içeren) Do() metoduna gönderildi
	resp, err := g.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tools []models.ToolSpec
	if err := json.NewDecoder(resp.Body).Decode(&tools); err != nil {
		return nil, err
	}
	return tools, nil
}

// --- 2. Adım: Araçları Ollama Formatına Çevir ---
func convertToolsForOllama(tools []models.ToolSpec) []models.OllamaTool {
	ollamaTools := make([]models.OllamaTool, len(tools))
	for i, tool := range tools {
		ollamaTools[i] = models.OllamaTool{
			Type: "function",
			Function: models.OllamaFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Schema,
			},
		}
	}
	return ollamaTools
}

// --- 4. Adım: Ollama'dan Gelen Tool Call'u Orchestrator'a Yönlendir ---
// GÜNCELLENDİ: Artık (json.RawMessage, int, error) olmak üzere 3 değer döndürüyor.
func (g *Gateway) callOrchestrator(ctx context.Context, toolCall models.OllamaToolCall) (json.RawMessage, int, error) {
	log.Printf("[Gateway] Ollama'dan gelen tool call Orchestrator'a yönlendiriliyor: %s", toolCall.Function.Name)

	rawArgs := json.RawMessage(toolCall.Function.Arguments)
	task := models.OrchestratorTaskRequest{
		AgentName: toolCall.Function.Name,
		Arguments: rawArgs,
	}

	reqBody, err := json.Marshal(task)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	// GÜNCELLENDİ: http.NewRequest yerine NewRequestWithContext kullanıldı
	req, err := http.NewRequestWithContext(ctx, "POST", g.Config.OrchestratorRunTaskURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	req.Header.Set("Content-Type", "application/json")

	// GÜNCELLENDİ: 'req' (context'i içeren) Do() metoduna gönderildi
	resp, err := g.HttpClient.Do(req)
	if err != nil {
		return nil, http.StatusServiceUnavailable, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	log.Printf("[Gateway] Orchestrator'dan yanıt alındı. Status: %d", resp.StatusCode)
	return body, resp.StatusCode, nil
}

// --- Ana Chat Handler ---
// GÜNCELLENDİ: Artık 200 OK (senkron) ve 202 Accepted (asenkron) durumlarını anlıyor.
// Dosya: llm_gateway/gateway.go

// --- Ana Chat Handler ---
func (g *Gateway) chatHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// --- ADIM 1: AUTHENTICATION (Kimlik Doğrulama) ---
	// ... (auth kodun burada, değişiklik yok) ...
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	userID, ok := g.authKeys[token]
	if !ok {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	// --- ADIM 2: İSTEĞİ OKUMA ---
	var chatReq ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("[Gateway] User %s new request: %s", userID, chatReq.Prompt)

	// --- ADIM 3: ARAÇLARI HAZIRLAMA ---
	tools, err := g.getToolsFromOrchestrator(ctx)
	if err != nil { /* ...hata kontrolü... */ }
	ollamaTools := convertToolsForOllama(tools)

	// --- ADIM 4: MESAJ LİSTESİNİ VE HAFIZAYI OLUŞTURMA (GÜNCELLENDİ) ---

	// 4a. Sistem Prompt'unu tanımla
	systemPrompt := `You are "Silka", an AI assistant created by Ozan Uslan.
Your primary goal is to be helpful and conversational.
You MUST ONLY use tools when the user explicitly asks for a specific action (like 'create', 'send', 'set', 'get') or mentions a tool name ('Slack', 'Calendar').
For simple greetings ('Hi', 'How are you?'), chit-chat, or memory questions ('what is my name?'), you MUST NOT use any tools. Just respond as a helpful assistant.`

	// 4b. Kullanıcının yeni mesajını oluştur
	newUserMessage := models.OllamaMessage{Role: "user", Content: chatReq.Prompt}

	// 4c. Geçmişi al (KİLİTLİ OKUMA)
	g.historyMu.RLock()
	userHistory := g.history[userID]
	g.historyMu.RUnlock()

	// 4d. TÜM MESAJLARI BİRLEŞTİR
	// Önce sistem prompt'u ile yeni bir liste başlat
	messagesForOllama := []models.OllamaMessage{
		{Role: "system", Content: systemPrompt},
	}
	// Sonra tüm geçmişi ekle
	messagesForOllama = append(messagesForOllama, userHistory...)
	// En sona o anki yeni mesajı ekle
	messagesForOllama = append(messagesForOllama, newUserMessage)

	// --- ADIM 5: OLLAMA'YI ÇAĞIRMA ---
	ollamaReq := models.OllamaChatRequest{
		Model:    g.Config.OllamaModel,
		Messages: messagesForOllama, // <-- ARTIK HEPSİNİ İÇERİYOR
		Tools:    ollamaTools,
		Stream:   false,
		KeepAlive: "1h",
	}

	// ... (Ollama'ya POST etme ve cevabı decode etme kısmı aynı) ...
	reqBody, _ := json.Marshal(ollamaReq)
	resp, err := g.HttpClient.Post(g.Config.OllamaURL, "application/json", bytes.NewBuffer(reqBody))
	// ... (hata kontrolü)

	if err != nil {
		log.Printf("Hata: Ollama'ya ulaşılamadı: %v", err)
		http.Error(w, "Ollama'ya ulaşılamadı", http.StatusInternalServerError)
		return // Hata varsa 'resp' nil'dir, 'defer' çağırmadan çık!
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Hata: Ollama'dan gelen yanıt OKUNAMADI: %v", err)
		http.Error(w, "Ollama yanıtı okunamadı", http.StatusInternalServerError)
		return
	}
	// HAM CEVABI LOGLA:
	log.Printf("[Gateway] Ollama HAM CEVABI: %s", string(bodyBytes))

	var ollamaResp models.OllamaChatResponse
	
	if err := json.Unmarshal(bodyBytes, &ollamaResp); err != nil {
		log.Printf("Hata: Ollama'dan gelen yanıt PARSE EDİLEMEDİ: %v", err)
		http.Error(w, "Ollama yanıtı anlaşılamadı", http.StatusInternalServerError)
		return
	}

	// --- ADIM 6: CEVABI İŞLEME VE HAFIZAYI GÜNCELLEME ---
	
	assistantMessageToSave := models.OllamaMessage{
		Role: "assistant",
	}

	if len(ollamaResp.Message.ToolCalls) > 0 {
		// DURUM 1: TOOL CALL VAR
		assistantMessageToSave.ToolCalls = ollamaResp.Message.ToolCalls

		g.historyMu.Lock()
		// GÜNCELLENDİ: Sadece 'user' ve 'assistant' mesajlarını kaydet (sistem prompt'unu değil)
		g.history[userID] = append(userHistory, newUserMessage, assistantMessageToSave)
		g.historyMu.Unlock()
		
		log.Printf("[Gateway] User %s için sohbet geçmişi (Tool Call) güncellendi.", userID)

		// 5. Orchestrator'ı çağır
		toolCall := ollamaResp.Message.ToolCalls[0]
		
		// callOrchestrator artık 3 değer dönüyor: body, status, error
		agentResult, statusCode, err := g.callOrchestrator(ctx, toolCall) 
		if err != nil {
			log.Printf("Hata: Orchestrator çağrılamadı: %v", err)
			http.Error(w, "Agent çalıştırılamadı", statusCode)
			return
		}

		// 6. Agent'ın sonucunu kullanıcıya dön
		w.Header().Set("Content-Type", "application/json")
		// Buraya da Content-Length eklemek iyi bir pratiktir
		w.Header().Set("Content-Length", strconv.Itoa(len(agentResult)))
		w.WriteHeader(statusCode)
		w.Write(agentResult)
		return

	} else {
		// DURUM 2: NORMAL METİN CEVABI VAR
		assistantMessageToSave.Content = ollamaResp.Message.Content

		g.historyMu.Lock()
		// GÜNCELLENDİ: Sadece 'user' ve 'assistant' mesajlarını kaydet (sistem prompt'unu değil)
		g.history[userID] = append(userHistory, newUserMessage, assistantMessageToSave)
		g.historyMu.Unlock()

		log.Printf("[Gateway] User %s için sohbet geçmişi (Metin) güncellendi.", userID)
		log.Println("[Gateway] Ollama'dan normal metin cevabı alındı.")

		// --- POSTMAN GÖRÜNMEZLİK SORUNU ÇÖZÜMÜ ---
		
		// 1. Cevabı hazırla
		responseMap := map[string]string{
			"response": ollamaResp.Message.Content,
		}

		// 2. Byte'a çevir
		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			http.Error(w, "Response encoding error", http.StatusInternalServerError)
			return
		}

		// 3. Headerları ayarla (ÖZELLİKLE CONTENT-LENGTH)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(responseBytes))) 
		w.WriteHeader(http.StatusOK)

		// 4. Gönder
		w.Write(responseBytes)
	}
}

// --- YENİ ENDPOINT ---
// handleChatStatus, kullanıcının bir görevin durumunu sorgulamasını sağlar.
func (g *Gateway) handleChatStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != "GET" {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. URL'den Task ID'yi ayıkla (örn: "/chat_status/abc-123" -> "abc-123")
	taskID := strings.TrimPrefix(r.URL.Path, "/chat_status/")
	if taskID == "" {
		http.Error(w, "Task ID eksik", http.StatusBadRequest)
		return
	}
	log.Printf("[Gateway] Durum sorgusu alındı: TaskID: %s", taskID)

	// 2. Orchestrator'ın durum sorgulama adresini oluştur
	// (örn: "http://localhost:8080/task_status/" + "abc-123")
	fullStatusURL := g.Config.OrchestratorToolsURL + "/../task_status/" + taskID
	// (Daha sağlamı: g.Config'e OrchestratorTaskStatusURL eklemek)

	req, err := http.NewRequestWithContext(ctx, "GET", fullStatusURL, nil)
	if err != nil {
		log.Printf("Hata: Orchestrator durum sorgu isteği oluşturulamadı: %v", err)
		http.Error(w, "Request creation failed", http.StatusInternalServerError)
		return
	}

	// 3. İsteği Orchestrator'a yönlendir (Proxy)
	resp, err := g.HttpClient.Do(req)
	if err != nil {
		log.Printf("Hata: Orchestrator durum sorgulanamadı: %v", err)
		http.Error(w, "Orchestrator status check failed", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// 4. Orchestrator'ın durum raporunu (TaskStatusResponse) kullanıcıya geri yolla
	log.Printf("[Gateway] Orchestrator durum yanıtı verdi: %s", resp.Status)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleChatStop, kullanıcının durdurma isteğini karşılar.
func (g *Gateway) handleChatStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/chat_stop/")
	log.Printf("[Gateway] Durdurma isteği: TaskID: %s", taskID)

	// Orchestrator'ın stop adresini oluştur
	baseURL := strings.Replace(g.Config.OrchestratorRunTaskURL, "/run_task", "/task_stop/", 1)
	fullStopURL := baseURL + taskID
	log.Printf("[Gateway] Orchestrator'a gidiliyor: %s", fullStopURL)

	// Orchestrator'a POST at
	req, _ := http.NewRequest("POST", fullStopURL, nil)
	resp, err := g.HttpClient.Do(req)
	log.Printf("[Gateway] Orchestrator stop yanıtı: %s, %s, %s", resp.Status, baseURL, fullStopURL)

	if err != nil {
		http.Error(w, "Orchestrator request failed", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func loadAuthKeys() map[string]string {
	keys := make(map[string]string)
	// Tüm environment değişkenlerini oku
	for _, env := range os.Environ() {
		// Bizimkilerle eşleşenleri bul
		if strings.HasPrefix(env, "API_KEY_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				keyName := parts[0] // "API_KEY_OZAN"
				token := parts[1]   // "ozan-secret-123"
				
				// "API_KEY_" önekini kaldırıp "ozan" (userID) elde et
				userID := strings.ToLower(strings.TrimPrefix(keyName, "API_KEY_")) 
				keys[token] = userID
				log.Printf("Yüklendi: API Key kullanıcısı: %s", userID)
			}
		}
	}
	return keys
}

// main fonksiyonu artık tüm sistemi "kurar" (setup)
func main() {
	if err := godotenv.Load("./../.env"); err != nil {
		// .env dosyasını llm_gateway klasöründe arar
		log.Println("Uyarı: .env dosyası bulunamadı, environment değişkenleri kullanılacak.")
	}

	cfg, err := NewGatewayConfig()
	if err != nil {
		log.Fatalf("Config yüklenemedi: %v", err)
	}

	authKeys := loadAuthKeys()
	if len(authKeys) == 0 {
		log.Println("UYARI: Hiçbir 'API_KEY_' .env'de bulunamadı. Kimlik doğrulama çalışmayacak.")
	}

	gateway := NewGateway(cfg, authKeys)

	// 3. Handler'ları (metodları) router'a kaydet
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", gateway.chatHandler)
	mux.HandleFunc("/chat_status/", gateway.handleChatStatus)
	mux.HandleFunc("/chat_stop/", gateway.handleChatStop)

	log.Printf("[LLM Gateway] Ana Backend servisi %s adresinde başlatılıyor...", cfg.ListenAddress)
	if err := http.ListenAndServe(cfg.ListenAddress, mux); err != nil {
		log.Fatalf("LLM Gateway başlatılamadı: %v", err)
	}
}
