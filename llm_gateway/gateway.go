package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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

type Gateway struct {
	GatewayConfig     *GatewayConfig
	HttpClient *http.Client
}

// NewGatewayConfig, .env dosyasını okur ve bir GatewayConfig struct'ı oluşturur.
func NewGatewayConfig() (*GatewayConfig, error) {
	timeoutStr := os.Getenv("HTTP_CLIENT_TIMEOUT_SECONDS")
	if timeoutStr == "" {
		timeoutStr = "60" // Varsayılan 60 saniye
	}
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("HTTP_CLIENT_TIMEOUT_SECONDS geçersiz: %v", err)
	}

	listenAddr := os.Getenv("GATEWAY_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8000" // Varsayılan port
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
		GatewayConfig: cfg,
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
// Artık 'Gateway' struct'ının bir metodu
func (g *Gateway) getToolsFromOrchestrator() ([]models.ToolSpec, error) {
	resp, err := g.HttpClient.Get(g.GatewayConfig.OrchestratorToolsURL) // <-- Global değişken yerine g.GatewayConfig ve g.HttpClient kullanır
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
func (g *Gateway) callOrchestrator(toolCall models.OllamaToolCall) (json.RawMessage, error) {
	log.Printf("[Gateway] Ollama'dan gelen tool call Orchestrator'a yönlendiriliyor: %s", toolCall.Function.Name)

	rawArgs := json.RawMessage(toolCall.Function.Arguments)

	task := models.OrchestratorTaskRequest{
		AgentName: toolCall.Function.Name,
		Arguments: rawArgs,
	}

	reqBody, err := json.Marshal(task)
	if err != nil {
		return nil, err
	}

	resp, err := g.HttpClient.Post(g.GatewayConfig.OrchestratorRunTaskURL, "application/json", bytes.NewBuffer(reqBody)) // <-- g.GatewayConfig ve g.HttpClient kullanır
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Printf("[Gateway] Orchestrator'dan yanıt alındı.")
	return body, nil
}

// --- Ana Chat Handler ---
func (g *Gateway) chatHandler(w http.ResponseWriter, r *http.Request) {
	var chatReq ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("[Gateway] Yeni chat isteği alındı: %s", chatReq.Prompt)

	// 1. Orchestrator'dan /tools listesini al
	tools, err := g.getToolsFromOrchestrator() // <-- Metod olarak çağır
	if err != nil {
		log.Printf("Hata: Orchestrator'dan tool listesi alınamadı: %v", err)
		http.Error(w, "Orchestrator'a ulaşılamadı", http.StatusInternalServerError)
		return
	}

	// 2. Araçları Ollama formatına çevir
	ollamaTools := convertToolsForOllama(tools)

	// 3. Ollama'ya isteği (prompt + tools) gönder
	ollamaReq := models.OllamaChatRequest{
		Model: g.GatewayConfig.OllamaModel, // <-- g.GatewayConfig'den al
		Messages: []models.OllamaMessage{
			{Role: "system", Content: "You are a helpful assistant that can use tools."},
			{Role: "user", Content: chatReq.Prompt},
		},
		Tools:     ollamaTools,
		Stream:    false,
		KeepAlive: "1h",
	}

	reqBody, _ := json.Marshal(ollamaReq)
	resp, err := g.HttpClient.Post(g.GatewayConfig.OllamaURL, "application/json", bytes.NewBuffer(reqBody)) // <-- g.GatewayConfig ve g.HttpClient kullanır
	if err != nil {
		log.Printf("Hata: Ollama'ya ulaşılamadı: %v", err)
		http.Error(w, "Ollama'ya ulaşılamadı", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var ollamaResp models.OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		log.Printf("Hata: Ollama'dan gelen yanıt parse edilemedi: %v", err)
		http.Error(w, "Ollama yanıtı anlaşılamadı", http.StatusInternalServerError)
		return
	}

	// 4. Ollama'nın Cevabını Değerlendir
	if len(ollamaResp.Message.ToolCalls) > 0 {
		toolCall := ollamaResp.Message.ToolCalls[0]

		// 5. Orchestrator'ı çağır
		agentResult, err := g.callOrchestrator(toolCall) // <-- Metod olarak çağır
		if err != nil {
			log.Printf("Hata: Orchestrator çağrılamadı: %v", err)
			http.Error(w, "Agent çalıştırılamadı", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
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

// main fonksiyonu artık tüm sistemi "kurar" (setup)
func main() {
	// ... (.env yükleme kısmı) ...
	if err := godotenv.Load("./../.env"); err != nil {
		log.Println("Uyarı: .env dosyası bulunamadı, environment değişkenleri kullanılacak.")
	}
	// 1. GatewayConfig'i oluştur (.env'den okur)
	cfg, err := NewGatewayConfig()
	if err != nil {
		log.Fatalf("GatewayConfig yüklenemedi: %v", err)
	}

	// 2. Gateway'i (GatewayConfig ve HttpClient ile) oluştur
	gateway := NewGateway(cfg)

	// 3. Handler'ları (metodları) router'a kaydet
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", gateway.chatHandler) // <-- 'gateway.chatHandler' metodunu kaydet

	// GÜNCELLENMİŞ LOG MESAJI (Printf kullanarak)
	log.Printf("[LLM Gateway] Ana Backend servisi %s adresinde başlatılıyor...", cfg.ListenAddress)

	// GÜNCELLENMİŞ SUNUCU BAŞLATMA
	if err := http.ListenAndServe(cfg.ListenAddress, mux); err != nil {
		log.Fatalf("LLM Gateway başlatılamadı: %v", err)
	}
}
