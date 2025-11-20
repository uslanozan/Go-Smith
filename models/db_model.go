package models

import "time"

// Her şeyi manual tanımlıyoruz

type User struct {
	ID        int       `gorm:"primaryKey;column:id"` 
	Username  string    `gorm:"column:username"`
	APIKey    string    `gorm:"column:api_key"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
	DeletedAt time.Time `gorm:"column:deleted_at"`
	
}

type Message struct {
	ID            int       `gorm:"primaryKey;column:id"`
	UserID        int       `gorm:"column:user_id"`
	Role          string    `gorm:"column:role"`
	Content       string    `gorm:"column:content"`
	ToolCallsJSON []byte    `gorm:"column:tool_calls_json"`
	
	CreatedAt     time.Time `gorm:"column:created_at"`
}