package main

import (
	"encoding/json"
)

type OllamaChatRequest struct {
	Model     string          `json:"model"`
	Messages  []OllamaMessage `json:"messages"`
	Tools     []OllamaTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream"`               // Cevabı tek parça halinde almak için
	KeepAlive string          `json:"keep_alive,omitempty"` // LLM'in kapanmaması için
}

type OllamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"`
}

type OllamaTool struct {
	Type     string         `json:"type"`
	Function OllamaFunction `json:"function"`
}

type OllamaFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type OllamaChatResponse struct {
	Message OllamaResponseMessage `json:"message"`
}

type OllamaResponseMessage struct {
	Content   string           `json:"content"`
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"`
}

type OllamaToolCall struct {
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"` // Ollama'dan gelen JSON verisindeki function ile buradaki Function structını birleştirir
}
