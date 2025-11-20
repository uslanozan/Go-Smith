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
	"time"

	"github.com/joho/godotenv"
	"github.com/uslanozan/Gollama-the-Orchestrator/models"
	"gorm.io/gorm"
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
	DB *gorm.DB
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
func NewGateway(cfg *GatewayConfig, db *gorm.DB) *Gateway {
	return &Gateway{
		Config: cfg,
		HttpClient: &http.Client{
			Timeout: cfg.HttpClientTimeout,
		},
		DB: db,
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

	// 1. AUTHENTICATION (DB'den)
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	
	var user models.User
	// DB'de API Key ara
	if result := g.DB.Where("api_key = ?", token).First(&user); result.Error != nil {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}
	log.Printf("[Gateway] Authenticated user: %s (ID: %d)", user.Username, user.ID)

	// 2. İSTEĞİ OKUMA
	var chatReq ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 3. ARAÇLARI HAZIRLAMA
	tools, _ := g.getToolsFromOrchestrator(ctx)
	ollamaTools := convertToolsForOllama(tools)

	// 4. HAFIZA (DB'den Çekme)
	var dbHistory []models.Message
	g.DB.Where("user_id = ?", user.ID).Order("created_at asc").Find(&dbHistory)

	var messagesForOllama []models.OllamaMessage
	// Sistem Prompt
	systemPrompt := `You are "Silka", an AI assistant created by Ozan Uslan. Your primary goal is to be helpful and conversational. You MUST ONLY use tools when the user explicitly asks for a specific action.`
	messagesForOllama = append(messagesForOllama, models.OllamaMessage{Role: "system", Content: systemPrompt})

	// MySQL'den çekilen Geçmiş Mesajlar
	for _, msg := range dbHistory {
		var toolCalls []models.OllamaToolCall
		if len(msg.ToolCallsJSON) > 0 {
			json.Unmarshal(msg.ToolCallsJSON, &toolCalls)
		}
		messagesForOllama = append(messagesForOllama, models.OllamaMessage{Role: msg.Role, Content: msg.Content, ToolCalls: toolCalls})
	}

	// Yeni Mesaj
	messagesForOllama = append(messagesForOllama, models.OllamaMessage{Role: "user", Content: chatReq.Prompt})

	// 5. OLLAMA ÇAĞRISI
	ollamaReq := models.OllamaChatRequest{
		Model: g.Config.OllamaModel, Messages: messagesForOllama, Tools: ollamaTools, Stream: false, KeepAlive: "1h",
	}
	reqBody, _ := json.Marshal(ollamaReq)
	resp, err := g.HttpClient.Post(g.Config.OllamaURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil { /* Hata */ return }
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var ollamaResp models.OllamaChatResponse
	json.Unmarshal(bodyBytes, &ollamaResp)

	// 6. KAYIT VE CEVAP
	
	// Kullanıcının sorusunu kaydet
	g.DB.Create(&models.Message{UserID: user.ID, Role: "user", Content: chatReq.Prompt})

	if len(ollamaResp.Message.ToolCalls) > 0 {
		// Tool Call Kaydet
		toolCalls := ollamaResp.Message.ToolCalls
		tcJSON, _ := json.Marshal(toolCalls)
		g.DB.Create(&models.Message{UserID: user.ID, Role: "assistant", ToolCallsJSON: tcJSON})
		
		// Orchestrator
		res, status, _ := g.callOrchestrator(ctx, toolCalls[0])
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(res)
	} else {
		// Normal Cevap Kaydet
		content := ollamaResp.Message.Content
		g.DB.Create(&models.Message{UserID: user.ID, Role: "assistant", Content: content})

		// Cevap Dön
		resMap := map[string]string{"response": content}
		resBytes, _ := json.Marshal(resMap)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(resBytes)))
		w.Write(resBytes)
	}
}

// handleChatStatus: Kullanıcının bir görevin durumunu sorgulamasını sağlar.
func (g *Gateway) handleChatStatus(w http.ResponseWriter, r *http.Request) {
	// 1. Context'i al (İptal sinyalleri için)
	ctx := r.Context()

	// 2. Sadece GET isteğine izin ver
	if r.Method != "GET" {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// 3. URL'den Task ID'yi ayıkla
	taskID := strings.TrimPrefix(r.URL.Path, "/chat_status/")
	if taskID == "" {
		http.Error(w, "Task ID eksik", http.StatusBadRequest)
		return
	}
	log.Printf("[Gateway] Durum sorgusu alındı: TaskID: %s", taskID)

	// 4. Orchestrator'ın adresini oluştur
	// (örn: "http://localhost:8080/task_status/" + "abc-123")
	fullStatusURL := g.Config.OrchestratorToolsURL + "/../task_status/" + taskID

	// 5. İsteği hazırla (Context ile birlikte)
	req, err := http.NewRequestWithContext(ctx, "GET", fullStatusURL, nil)
	if err != nil {
		log.Printf("Hata: İstek oluşturulamadı: %v", err)
		http.Error(w, "Request creation failed", http.StatusInternalServerError)
		return
	}

	// 6. İsteği Orchestrator'a gönder (CRITICAL FIX BURADA)
	resp, err := g.HttpClient.Do(req)
	
	// HATA KONTROLÜ ÖNCE YAPILMALI:
	if err != nil {
		log.Printf("Hata: Orchestrator durum sorgulanamadı: %v", err)
		http.Error(w, "Orchestrator status check failed", http.StatusServiceUnavailable)
		return
	}
	// Hata yoksa 'defer' güvenlidir
	defer resp.Body.Close()

	// 7. Orchestrator'dan gelen cevabı kullanıcıya yansıt (Proxy)
	log.Printf("[Gateway] Orchestrator durum yanıtı verdi: %s", resp.Status)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleChatStop: Kullanıcının durdurma isteğini karşılar.
func (g *Gateway) handleChatStop(w http.ResponseWriter, r *http.Request) {
	// 1. Sadece POST isteğine izin ver
	if r.Method != "POST" {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. URL'den Task ID'yi ayıkla
	taskID := strings.TrimPrefix(r.URL.Path, "/chat_stop/")
	log.Printf("[Gateway] Durdurma isteği: TaskID: %s", taskID)

	// 3. Orchestrator'ın stop adresini oluştur (String replace ile)
	baseURL := strings.Replace(g.Config.OrchestratorRunTaskURL, "/run_task", "/task_stop/", 1)
	fullStopURL := baseURL + taskID
	
	log.Printf("[Gateway] Orchestrator'a gidiliyor: %s", fullStopURL)

	// 4. Orchestrator'a POST at
	req, _ := http.NewRequest("POST", fullStopURL, nil)
	
	// 5. İsteği gönder (CRITICAL FIX BURADA)
	resp, err := g.HttpClient.Do(req)

	// HATA KONTROLÜ ÖNCE YAPILMALI:
	if err != nil {
		log.Printf("Hata: Orchestrator'a durdurma isteği atılamadı: %v", err)
		http.Error(w, "Orchestrator request failed", http.StatusServiceUnavailable)
		return
	}
	// Hata yoksa 'defer' güvenlidir
	defer resp.Body.Close()

	log.Printf("[Gateway] Orchestrator stop yanıtı: %s", resp.Status)

	// 6. Cevabı kullanıcıya yansıt
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// main fonksiyonu artık tüm sistemi "kurar" (setup)
func main() {
	if err := godotenv.Load("./../.env"); err != nil {
		log.Println("Uyarı: .env dosyası bulunamadı, environment değişkenleri kullanılacak.")
	}

	cfg, err := NewGatewayConfig()
	if err != nil {
		log.Fatalf("Config yüklenemedi: %v", err)
	}

	db, err := InitDB()
    if err != nil {
        log.Fatalf("DB Hatası: %v", err)
    }

	gateway := NewGateway(cfg, db)

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", gateway.chatHandler)
	mux.HandleFunc("/chat_status/", gateway.handleChatStatus)
	mux.HandleFunc("/chat_stop/", gateway.handleChatStop)

	log.Printf("[LLM Gateway] Ana Backend servisi %s adresinde başlatılıyor...", cfg.ListenAddress)
	if err := http.ListenAndServe(cfg.ListenAddress, mux); err != nil {
		log.Fatalf("LLM Gateway başlatılamadı: %v", err)
	}
}
