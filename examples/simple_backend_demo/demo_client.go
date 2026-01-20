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
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/uslanozan/Go-Smith/models"
	"gorm.io/gorm"
)

type GatewayConfig struct {
	OllamaURL              string
	OrchestratorToolsURL   string
	OrchestratorRunTaskURL string
	OllamaModel            string
	HttpClientTimeout      time.Duration
	ListenAddress          string
}

type Gateway struct {
	Config     *GatewayConfig
	HttpClient *http.Client
	DB         *gorm.DB
}

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

func NewGateway(cfg *GatewayConfig, db *gorm.DB) *Gateway {
	return &Gateway{
		Config: cfg,
		HttpClient: &http.Client{
			Timeout: cfg.HttpClientTimeout,
		},
		DB: db,
	}
}

type ChatRequest struct {
	Prompt string `json:"prompt"`
}

func (g *Gateway) getToolsFromOrchestrator(ctx context.Context) ([]models.ToolSpec, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", g.Config.OrchestratorToolsURL, nil)
	if err != nil {
		return nil, err
	}

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

func convertToolsForOllama(tools []models.ToolSpec) []OllamaTool {
	ollamaTools := make([]OllamaTool, len(tools))
	for i, tool := range tools {
		ollamaTools[i] = OllamaTool{
			Type: "function",
			Function: OllamaFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Schema,
			},
		}
	}
	return ollamaTools
}

func (g *Gateway) callOrchestrator(ctx context.Context, toolCall OllamaToolCall) (json.RawMessage, int, error) {
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

	req, err := http.NewRequestWithContext(ctx, "POST", g.Config.OrchestratorRunTaskURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	req.Header.Set("Content-Type", "application/json")

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

func (g *Gateway) chatHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	var user User

	if result := g.DB.Where("api_key = ?", token).First(&user); result.Error != nil {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}
	log.Printf("[Gateway] Authenticated user: %s (ID: %d)", user.Username, user.ID)

	var chatReq ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tools, _ := g.getToolsFromOrchestrator(ctx)
	ollamaTools := convertToolsForOllama(tools)

	var dbHistory []Message

	limit := 20
	g.DB.Where("user_id = ?", user.ID).
		Order("created_at desc").
		Limit(limit).
		Find(&dbHistory)

	// LLM yukarıdan aşağı okuduğu için listeyi ters çeviriyoruz eskiden yeniye
	for i, j := 0, len(dbHistory)-1; i < j; i, j = i+1, j-1 {
		dbHistory[i], dbHistory[j] = dbHistory[j], dbHistory[i]
	}

	var messagesForOllama []OllamaMessage
	systemPrompt := `You are "Silka", an AI assistant created by Ozan Uslan. Your primary goal is to be helpful and conversational. You MUST ONLY use tools when the user explicitly asks for a specific action.`
	messagesForOllama = append(messagesForOllama, OllamaMessage{Role: "system", Content: systemPrompt})

	for _, msg := range dbHistory {
		var toolCalls []OllamaToolCall
		if len(msg.ToolCallsJSON) > 0 {
			json.Unmarshal(msg.ToolCallsJSON, &toolCalls)
		}
		messagesForOllama = append(messagesForOllama, OllamaMessage{Role: msg.Role, Content: msg.Content, ToolCalls: toolCalls})
	}

	messagesForOllama = append(messagesForOllama, OllamaMessage{Role: "user", Content: chatReq.Prompt})

	ollamaReq := OllamaChatRequest{
		Model: g.Config.OllamaModel, Messages: messagesForOllama, Tools: ollamaTools, Stream: false, KeepAlive: "1h",
	}
	reqBody, _ := json.Marshal(ollamaReq)
	resp, err := g.HttpClient.Post(g.Config.OllamaURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var ollamaResp OllamaChatResponse
	json.Unmarshal(bodyBytes, &ollamaResp)

	g.DB.Create(&Message{UserID: user.ID, Role: "user", Content: chatReq.Prompt})

	if len(ollamaResp.Message.ToolCalls) > 0 {
		toolCalls := ollamaResp.Message.ToolCalls
		tcJSON, _ := json.Marshal(toolCalls)
		g.DB.Create(&Message{UserID: user.ID, Role: "assistant", ToolCallsJSON: tcJSON})

		res, status, _ := g.callOrchestrator(ctx, toolCalls[0])
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(res)
	} else {
		content := ollamaResp.Message.Content
		g.DB.Create(&Message{UserID: user.ID, Role: "assistant", Content: content})

		resMap := map[string]string{"response": content}
		resBytes, _ := json.Marshal(resMap)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(resBytes)))
		w.Write(resBytes)
	}
}

func (g *Gateway) handleChatStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != "GET" {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/chat_status/")
	if taskID == "" {
		http.Error(w, "Task ID eksik", http.StatusBadRequest)
		return
	}
	log.Printf("[Gateway] Durum sorgusu alındı: TaskID: %s", taskID)

	fullStatusURL := g.Config.OrchestratorToolsURL + "/../task_status/" + taskID

	req, err := http.NewRequestWithContext(ctx, "GET", fullStatusURL, nil)
	if err != nil {
		log.Printf("Hata: İstek oluşturulamadı: %v", err)
		http.Error(w, "Request creation failed", http.StatusInternalServerError)
		return
	}

	resp, err := g.HttpClient.Do(req)

	if err != nil {
		log.Printf("Hata: Orchestrator durum sorgulanamadı: %v", err)
		http.Error(w, "Orchestrator status check failed", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	log.Printf("[Gateway] Orchestrator durum yanıtı verdi: %s", resp.Status)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (g *Gateway) handleChatStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/chat_stop/")
	log.Printf("[Gateway] Durdurma isteği: TaskID: %s", taskID)

	baseURL := strings.Replace(g.Config.OrchestratorRunTaskURL, "/run_task", "/task_stop/", 1)
	fullStopURL := baseURL + taskID

	log.Printf("[Gateway] Orchestrator'a gidiliyor: %s", fullStopURL)

	req, _ := http.NewRequest("POST", fullStopURL, nil)

	resp, err := g.HttpClient.Do(req)

	if err != nil {
		log.Printf("Hata: Orchestrator'a durdurma isteği atılamadı: %v", err)
		http.Error(w, "Orchestrator request failed", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	log.Printf("[Gateway] Orchestrator stop yanıtı: %s", resp.Status)

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func main() {
	if err := godotenv.Load("./../../.env"); err != nil {
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
