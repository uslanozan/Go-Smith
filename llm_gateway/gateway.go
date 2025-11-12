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
func NewGateway(cfg *GatewayConfig) *Gateway {
	return &Gateway{
		Config: cfg,
		HttpClient: &http.Client{
			Timeout: cfg.HttpClientTimeout,
		},
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
func (g *Gateway) chatHandler(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var chatReq ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("[Gateway] Yeni chat isteği alındı: %s", chatReq.Prompt)

	// ... (1. Adım: getToolsFromOrchestrator ve 2. Adım: convertToolsForOllama aynı) ...
	tools, err := g.getToolsFromOrchestrator(ctx)
	if err != nil { /* ...hata kontrolü... */ }
	ollamaTools := convertToolsForOllama(tools)

	// 3. Ollama'ya isteği (prompt + tools) gönder
	ollamaReq := models.OllamaChatRequest{
		Model: g.Config.OllamaModel,
		Messages: []models.OllamaMessage{
			{Role: "system", Content: "You are a helpful assistant that can use tools."},
			{Role: "user", Content: chatReq.Prompt},
		},
		Tools:     ollamaTools,
		Stream:    false,
		KeepAlive: "1h",
	}
	// ... (Ollama'ya POST etme ve cevabı decode etme kısmı aynı) ...
	reqBody, _ := json.Marshal(ollamaReq)
	resp, err := g.HttpClient.Post(g.Config.OllamaURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil { /* ...hata kontrolü... */ }
	defer resp.Body.Close()
	var ollamaResp models.OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil { /* ...hata kontrolü... */ }

	// 4. Ollama'nın Cevabını Değerlendir
	if len(ollamaResp.Message.ToolCalls) > 0 {
		toolCall := ollamaResp.Message.ToolCalls[0]

		// 5. Orchestrator'ı çağır
		// Artık 3 değer alıyoruz: sonuç, durum kodu, hata
		agentResult, statusCode, err := g.callOrchestrator(ctx, toolCall)		
		if err != nil {
			log.Printf("Hata: Orchestrator çağrılamadı: %v", err)
			http.Error(w, "Agent çalıştırılamadı", statusCode) // Dönen hatayı yansıt
			return
		}

		// 6. Agent'ın sonucunu kullanıcıya dön
		// (Bu artık 200 OK (senkron sonuç) VEYA 202 Accepted (asenkron TaskID) olabilir)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode) // Orchestrator'dan gelen kodu (200 veya 202) yansıt
		w.Write(agentResult)
		return
	}

	// EĞER NORMAL METİN CEVABI VARSA:
	log.Println("[Gateway] Ollama'dan normal metin cevabı alındı.")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"response": ollamaResp.Message.Content,
	})
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

	gateway := NewGateway(cfg)

	// 3. Handler'ları (metodları) router'a kaydet
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", gateway.chatHandler)
	mux.HandleFunc("/chat_status/", gateway.handleChatStatus) // <-- YENİ ENDPOINT KAYDI

	log.Printf("[LLM Gateway] Ana Backend servisi %s adresinde başlatılıyor...", cfg.ListenAddress)
	if err := http.ListenAndServe(cfg.ListenAddress, mux); err != nil {
		log.Fatalf("LLM Gateway başlatılamadı: %v", err)
	}
}