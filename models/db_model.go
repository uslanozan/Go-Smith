// Dosya: models/db_models.go

package models

import (
	"time"

	"gorm.io/gorm"
)

//todo: indexler atılacak sonra eklenecek

// Tablo: user
type User struct {
	// Field'lar
	ID int `gorm:"primaryKey;column:id;autoIncrement;unique;not null;type:int"`
	Username string `gorm:"column:username;type:varchar(100);unique;not null;index"`
	APIKey string `gorm:"column:api_key;type:varchar(255);unique;not null;index"`
	Email string `gorm:"column:email;type:varchar(255)"`
	PasswordHash string `gorm:"column:password_hash;type:varchar(255)"`
	Name    string `gorm:"column:name;type:varchar(100);not null"`
	Surname string `gorm:"column:surname;type:varchar(100);not null"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
	
	// Relation'lar
	UserAgents []UserAgent `gorm:"foreignKey:UserID"`
	Messages   []Message   `gorm:"foreignKey:UserID"`
}

// Tablo: agent
type Agent struct {
	// Field'lar
	ID int `gorm:"primaryKey;column:id;autoIncrement;unique;not null;type:int"`
	AgentName string `gorm:"column:agent_name;type:varchar(100);unique;not null"`
	Description string `gorm:"column:description;type:mediumtext"`
	Status int `gorm:"column:status;type:tinyint;not null;default:1"`
	Endpoint string `gorm:"column:endpoint;type:varchar(255);not null"`
	StopEndpoint   string `gorm:"column:stop_endpoint;type:varchar(255)"`
	StatusEndpoint string `gorm:"column:status_endpoint;type:varchar(255)"`
	SchemaJSON []byte `gorm:"column:schema_json;type:json"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP"`

	// Relation'lar
	Functions []AgentFunction `gorm:"foreignKey:AgentID"`
}

// Tablo: agent_function
type AgentFunction struct {
	// Field'lar
	ID int `gorm:"primaryKey;column:id;autoIncrement;unique;not null;type:int"`
	FunctionName        string `gorm:"column:function_name;type:varchar(45);unique;not null"`
	FunctionDescription string `gorm:"column:function_description;type:mediumtext"`
	AgentID int `gorm:"column:agent_id;not null"`

	// Relation'lar
	Agent   Agent `gorm:"foreignKey:AgentID"`
}

// Tablo: user_agent
type UserAgent struct {
	// Field'lar
	UserID  int `gorm:"primaryKey;column:user_id;not null;index:idx_user_agent_composite,unique;type:int"`
	AgentID int `gorm:"primaryKey;column:agent_id;not null;index:idx_user_agent_composite,unique;type:int"`
	ConfigJSON []byte `gorm:"column:config_json;type:json"`
	IsActive int `gorm:"column:is_active;type:tinyint;not null;default:1"`
	ActivatedAt   time.Time `gorm:"column:activated_at;type:datetime;default:CURRENT_TIMESTAMP"`
	DeactivatedAt time.Time `gorm:"column:deactivated_at;type:datetime"`

	// Relation'lar
	User  User  `gorm:"foreignKey:UserID"`
    Agent Agent `gorm:"foreignKey:AgentID"`
}

// Tablo: message
type Message struct {
	// Field'lar
	ID int `gorm:"primaryKey;column:id;autoIncrement;unique;not null;type:int"`
	Role string `gorm:"column:role;type:varchar(45);not null"`
	Content string `gorm:"column:content;type:longtext"`
	ToolCallsJSON []byte `gorm:"column:tool_calls;type:json"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;index"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
	UserID int `gorm:"column:user_id;not null;index"`

	// Relation'lar
	User   User `gorm:"foreignKey:UserID"`
}