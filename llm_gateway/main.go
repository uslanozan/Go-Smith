package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

// Orchestrator'ımızın /tools endpoint'inden dönen format
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

// Ollama'ya göndereceğimiz /api/chat request'inin formatı
type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Tools    []OllamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
}

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Ollama'nın tool tanımı formatı
type OllamaTool struct {
	Type     string         `json:"type"`
	Function OllamaFunction `json:"function"`
}

type OllamaFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Ollama'dan gelen /api/chat response'unun formatı
type OllamaChatResponse struct {
	Message OllamaResponseMessage `json:"message"`
}

type OllamaResponseMessage struct {
	Content   string           `json:"content"` // Normal metin cevabı
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"` // Araç çağırma isteği
}

// Ollama'nın araç çağırma formatı
type OllamaToolCall struct {
	Function struct {
		Name      string `json:"name"`
		// ARTIK JSON.RawMessage KULLANIYORUZ, çÜNKÜ MODEL OBJE GÖNDERİYOR
		Arguments json.RawMessage `json:"arguments"` 
	} `json:"function"`
}

// Orchestrator'a göndereceğimiz /run_task formatı
type OrchestratorTaskRequest struct {
	AgentName string          `json:"agent_name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Kullanıcıdan /chat endpoint'ine gelecek format
type ChatRequest struct {
	Prompt string `json:"prompt"`
}

var httpClient = &http.Client{Timeout: 60 * time.Second}
var ollamaURL = "http://localhost:11434/api/chat"
var orchestratorToolsURL = "http://localhost:8080/tools"
var orchestratorRunTaskURL = "http://localhost:8080/run_task"
var ollamaModel = "llama3.1:8b-instruct-q8_0" // Veya tool-use destekleyen başka bir model

// --- 1. Adım: Orchestrator'dan Araç Listesini Al ---
func getToolsFromOrchestrator() ([]ToolSpec, error) {
	resp, err := httpClient.Get(orchestratorToolsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tools []ToolSpec
	if err := json.NewDecoder(resp.Body).Decode(&tools); err != nil {
		return nil, err
	}
	return tools, nil
}

// --- 2. Adım: Araçları Ollama Formatına Çevir ---
func convertToolsForOllama(tools []ToolSpec) []OllamaTool {
	ollamaTools := make([]OllamaTool, len(tools))
	for i, tool := range tools {
		ollamaTools[i] = OllamaTool{
			Type: "function",
			Function: OllamaFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Schema, // Bizim schema'mız Ollama'nın 'parameters' objesine tam uyuyor
			},
		}
	}
	return ollamaTools
}

// --- 4. Adım: Ollama'dan Gelen Tool Call'u Orchestrator'a Yönlendir ---
func callOrchestrator(toolCall OllamaToolCall) (json.RawMessage, error) {
	log.Printf("[Gateway] Ollama'dan gelen tool call Orchestrator'a yönlendiriliyor: %s", toolCall.Function.Name)

	// Ollama'dan string olarak gelen argümanları RawMessage'a çevir
	// Bu, {"agent_name": ..., "arguments": "{...}"} yerine {"agent_name": ..., "arguments": {...}} olmasını sağlar
	rawArgs := json.RawMessage(toolCall.Function.Arguments)

	task := OrchestratorTaskRequest{
		AgentName: toolCall.Function.Name,
		Arguments: rawArgs,
	}

	reqBody, err := json.Marshal(task)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Post(orchestratorRunTaskURL, "application/json", bytes.NewBuffer(reqBody))
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
func chatHandler(w http.ResponseWriter, r *http.Request) {
	var chatReq ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("[Gateway] Yeni chat isteği alındı: %s", chatReq.Prompt)

	// 1. Orchestrator'dan /tools listesini al
	tools, err := getToolsFromOrchestrator()
	if err != nil {
		log.Printf("Hata: Orchestrator'dan tool listesi alınamadı: %v", err)
		http.Error(w, "Orchestrator'a ulaşılamadı", http.StatusInternalServerError)
		return
	}

	// 2. Araçları Ollama formatına çevir
	ollamaTools := convertToolsForOllama(tools)

	// 3. Ollama'ya isteği (prompt + tools) gönder
	ollamaReq := OllamaChatRequest{
		Model: ollamaModel,
		Messages: []OllamaMessage{
			{Role: "system", Content: "You are a helpful assistant that can use tools."},
			{Role: "user", Content: chatReq.Prompt},
		},
		Tools:  ollamaTools,
		Stream: false,
	}

	reqBody, _ := json.Marshal(ollamaReq)
	resp, err := httpClient.Post(ollamaURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Hata: Ollama'ya ulaşılamadı: %v", err)
		http.Error(w, "Ollama'ya ulaşılamadı", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var ollamaResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		log.Printf("Hata: Ollama'dan gelen yanıt parse edilemedi: %v", err)
		http.Error(w, "Ollama yanıtı anlaşılamadı", http.StatusInternalServerError)
		return
	}

	// 4. Ollama'nın Cevabını Değerlendir
	
	// EĞER TOOL CALL (ARAÇ ÇAĞIRMA) VARSA:
	if ollamaResp.Message.ToolCalls != nil && len(ollamaResp.Message.ToolCalls) > 0 {
		toolCall := ollamaResp.Message.ToolCalls[0] // Şimdilik sadece ilk tool call'u çalıştıralım
		
		// 5. Orchestrator'ı çağır
		agentResult, err := callOrchestrator(toolCall)
		if err != nil {
			log.Printf("Hata: Orchestrator çağrılamadı: %v", err)
			http.Error(w, "Agent çalıştırılamadı", http.StatusInternalServerError)
			return
		}
		
		// 6. Agent'ın sonucunu kullanıcıya dön
		// (Normalde bu sonucu tekrar LLM'e gönderip özetletmek daha iyi olur,
		// ama şimdilik direkt agent sonucunu dönelim)
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

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", chatHandler)

	log.Println("[LLM Gateway] Ana Backend servisi http://localhost:8000 adresinde başlatılıyor...")
	if err := http.ListenAndServe(":8000", mux); err != nil {
		log.Fatalf("LLM Gateway başlatılamadı: %v", err)
	}
}