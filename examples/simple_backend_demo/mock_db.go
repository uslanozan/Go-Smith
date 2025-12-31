package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/uslanozan/Go-Smith/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func InitDB() (*gorm.DB, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("DB_DSN environment içinde tanımlanmamış, database oluşturulamadı")
	}

	parts := strings.Split(dsn, "/")
	if len(parts) > 1 {
		rootDSN := parts[0] + "/"

		if strings.Contains(parts[1], "?") {
			params := strings.Split(parts[1], "?")[1]
			rootDSN += "?" + params
		}

		dbName := strings.Split(strings.Split(parts[1], "?")[0], " ")[0]

		tempDB, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{})
		if err == nil {
			log.Printf("Veritabanı kontrol ediliyor: %s", dbName)
			tempDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName))
			sqlDB, _ := tempDB.DB()
			sqlDB.Close()
		}
	}

	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	}

	db, err := gorm.Open(mysql.Open(dsn), config)
	if err != nil {
		return nil, fmt.Errorf("MySQL bağlantı hatası: %v", err)
	}

	log.Println("MySQL bağlantısı başarılı.")

	if os.Getenv("DB_AUTO_MIGRATE") == "true" {
		log.Println("🔄 AutoMigrate çalıştırılıyor...")
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
		log.Println("✅ Tablolar senkronize edildi.")
	} else {
		log.Println("ℹ️ AutoMigrate atlandı (DB_AUTO_MIGRATE != true).")
	}

	if db.Migrator().HasTable(&models.User{}) {
		var count int64
		db.Model(&models.User{}).Where("api_key = ?", "demo-token-123").Count(&count)
		if count == 0 {
			demoUser := models.User{
				Username: "demo_user",
				APIKey:   "demo-token-123",
			}
			if err := db.Create(&demoUser).Error; err == nil {
				log.Println("👤 Demo kullanıcı oluşturuldu. (Token: demo-token-123)")
			}
		}
	}

	return db, nil
}
