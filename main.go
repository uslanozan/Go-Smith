package main

import (
	"log"
	"net/http"
)

func main() {
	// 1. Agent Kayıt Defterini oluştur
	registry := NewAgentRegistry()

	// 2. Agent'ları koddan değil, config dosyasından yükle
	//todo: Gelecekte buradaki config'i backendden alacak
	if err := LoadAgentsFromConfig(registry, "config/agents.json"); err != nil {
		log.Fatalf("Agent konfigürasyonu yüklenemedi: %v", err)
	}

	taskRegistry := NewTaskRegistry()

	// 3. Orchestrator'ı oluştur
	orchestrator := NewOrchestrator(registry, taskRegistry)

	// 4. HTTP sunucu ayarları
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tools", orchestrator.HandleGetTools)
	mux.HandleFunc("/api/v1/run_task", orchestrator.HandleTask)
	mux.HandleFunc("/api/v1/task_status/", orchestrator.HandleTaskStatus)
	mux.HandleFunc("/api/v1/task_stop/", orchestrator.HandleTaskStop)

	log.Println("Go Orchestrator sunucusu http://localhost:8080 adresinde başlatılıyor...")

	// 5. Sunucuyu başlat
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Sunucu başlatılamadı: %v", err)
	}
}
