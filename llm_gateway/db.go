package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// InitDB: Veritabanına bağlanır ve tabloları oluşturur
func InitDB() (*gorm.DB, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("DB_DSN environment variable not set")
	}

	// log ayarı
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // Default olarak çoğul yapıyor bunu eklememiz gerekli
		},
	}

	db, err := gorm.Open(mysql.Open(dsn), config)
	if err != nil {
		return nil, fmt.Errorf("veritabanına bağlanılamadı: %v", err)
	}

	log.Println("MySQL bağlantısı başarılı.")


	log.Println("Veritabanı tabloları (User, Message) başarıyla senkronize edildi.")
	
	return db, nil
}