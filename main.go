package main

import (
	"log"
	"net/http"
)

func main() {
	// 1. Agent Kayıt Defterini oluştur
	registry := NewAgentRegistry()

	// 2. Agent'ları koddan değil, config dosyasından yükle
	if err := LoadAgentsFromConfig(registry, "config/agents.json"); err != nil {
		log.Fatalf("Agent konfigürasyonu yüklenemedi: %v", err)
	}

	taskRegistry := NewTaskRegistry()

	// 3. Orchestrator'ı (dağıtıcıyı) oluştur
	orchestrator := NewOrchestrator(registry, taskRegistry)

	// 4. HTTP sunucu ayarları
	mux := http.NewServeMux()
	mux.HandleFunc("/tools", orchestrator.HandleGetTools)
	mux.HandleFunc("/run_task", orchestrator.HandleTask)
	mux.HandleFunc("/task_status/", orchestrator.HandleTaskStatus)

	log.Println("Go Orchestrator sunucusu http://localhost:8080 adresinde başlatılıyor...")

	// 5. Sunucuyu başlat
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Sunucu başlatılamadı: %v", err)
	}
}
