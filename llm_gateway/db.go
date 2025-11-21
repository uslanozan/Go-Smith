// Dosya: llm_gateway/db.go

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/uslanozan/Gollama-the-Orchestrator/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// InitDB: Veritabanını yoksa oluşturur, bağlanır ve tabloları hazırlar.
func InitDB() (*gorm.DB, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("DB_DSN environment içinde tanımlanmamış, database oluşturulamadı")
	}

	// Veritabanı var mı diye bakar
	parts := strings.Split(dsn, "/")
	if len(parts) > 1 {
		rootDSN := parts[0] + "/" // "user:pass@tcp(addr)/"

		if strings.Contains(parts[1], "?") {
			params := strings.Split(parts[1], "?")[1]
			rootDSN += "?" + params
		}

		// Veritabanı ismini alır
		dbName := strings.Split(strings.Split(parts[1], "?")[0], " ")[0]

		// Root'a geçici bağlan
		tempDB, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{})
		if err == nil {
			log.Printf("Veritabanı kontrol ediliyor: %s", dbName)
			// Veritabanını oluştur (Eğer yoksa)
			// Not: Veritabanı adı SQL Injection riski taşımadığı sürece Sprintf güvenlidir,
			// ama prodüksiyonda dikkatli olunmalı. Burada dbName .env'den geliyor.
			tempDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName))

			// Bağlantıyı kapat
			sqlDB, _ := tempDB.DB()
			sqlDB.Close()
		}
	}

	// --- ADIM 2: ASIL BAĞLANTI ---
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // Tablo isimlerini çoğul yapma (User -> user)
		},
	}

	db, err := gorm.Open(mysql.Open(dsn), config)
	if err != nil {
		return nil, fmt.Errorf("MySQL bağlantı hatası: %v", err)
	}

	log.Println("MySQL bağlantısı başarılı.")

	// --- ADIM 3: AUTO MIGRATE (Senin Kurallarınla) ---
	// Bu komut, models klasöründeki struct'lara göre tabloları oluşturur.
	// Seed yapmaz, sadece şemayı hazırlar.
	err = db.AutoMigrate(
		&models.User{},
		&models.Agent{},
		&models.UserAgent{},
		&models.AgentFunction{},
		&models.Message{},
	)
	if err != nil {
		return nil, fmt.Errorf("tablo oluşturma hatası: %v", err)
	}

	log.Println("Tablolar (User, Agent, UserAgent, AgentFunction, Message) senkronize edildi.")

	return db, nil
}
