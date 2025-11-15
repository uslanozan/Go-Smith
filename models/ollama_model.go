package models

import (
	"encoding/json"
)

// Ollama'ya göndereceğimiz /api/chat request'inin formatı
type OllamaChatRequest struct {
	Model     string          `json:"model"`
	Messages  []OllamaMessage `json:"messages"`
	Tools     []OllamaTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream"` // Cevabı tek parça halinde almak için
	KeepAlive string          `json:"keep_alive,omitempty"` // LLM'in kapanmaması için
}

// Mesaj formatı
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"`
}

// Ollama'nın tool tanımı formatı
type OllamaTool struct {
	Type     string         `json:"type"`
	Function OllamaFunction `json:"function"`
}

// Agent fonksiyonları
type OllamaFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Ollama'dan gelen /api/chat response'unun formatı
type OllamaChatResponse struct {
	Message OllamaResponseMessage `json:"message"`
}

// Cevap içeriği
type OllamaResponseMessage struct {
	Content   string           `json:"content"`              // Normal metin cevabı
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"` // Araç çağırma isteği
}

// Ollama'nın araç çağırma formatı
type OllamaToolCall struct {
	Function struct {
		Name string `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`  // Ollama'dan gelen JSON verisindeki function ile buradaki Function structını birleştirir
}