// Dosya: models/db_models.go

package models

import (
	"time"

	"gorm.io/gorm"
)

// Tablo: user
type User struct {
	// SQL: id INT NOT NULL AUTO_INCREMENT, UNIQUE INDEX ID_UNIQUE
	ID int32 `gorm:"primaryKey;column:id;autoIncrement;not null;uniqueIndex:ID_UNIQUE"`

	// SQL: UNIQUE INDEX Username_UNIQUE
	Username string `gorm:"column:username;type:varchar(100);not null;uniqueIndex:Username_UNIQUE"`

	// SQL: UNIQUE INDEX APIKey_UNIQUE
	APIKey string `gorm:"column:api_key;type:varchar(255);not null;uniqueIndex:APIKey_UNIQUE"`

	// SQL: UNIQUE INDEX email_UNIQUE
	Email string `gorm:"column:email;type:varchar(255);not null;uniqueIndex:email_UNIQUE"`

	PasswordHash string `gorm:"column:password_hash;type:varchar(255);not null"`
	Name         string `gorm:"column:name;type:varchar(100);not null"`
	Surname      string `gorm:"column:surname;type:varchar(100);not null"`

	CreatedAt time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP"`

	// SQL: INDEX idx_user_deleted_at
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index:idx_user_deleted_at"`

	// Relation'lar
	UserAgents []UserAgent `gorm:"foreignKey:UserID"`
	Messages   []Message   `gorm:"foreignKey:UserID"`
}

// Tablo: agent
type Agent struct {
	// SQL: UNIQUE INDEX id_UNIQUE
	ID int32 `gorm:"primaryKey;column:id;autoIncrement;not null;uniqueIndex:id_UNIQUE"`

	// SQL: UNIQUE INDEX agent_name_UNIQUE
	AgentName string `gorm:"column:agent_name;type:varchar(100);not null;uniqueIndex:agent_name_UNIQUE"`

	Description    string `gorm:"column:description;type:mediumtext"`
	Status         int    `gorm:"column:status;type:tinyint;not null;default:1"`
	Endpoint       string `gorm:"column:endpoint;type:varchar(255);not null"`
	StopEndpoint   string `gorm:"column:stop_endpoint;type:varchar(255)"`
	StatusEndpoint string `gorm:"column:status_endpoint;type:varchar(255)"`
	SchemaJSON     []byte `gorm:"column:schema_json;type:json"`

	CreatedAt time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP"`

	// Relation'lar
	Functions []AgentFunction `gorm:"foreignKey:AgentID"`
}

// Tablo: agent_function
type AgentFunction struct {
	// SQL: UNIQUE INDEX id_UNIQUE
	ID int32 `gorm:"primaryKey;column:id;autoIncrement;not null;uniqueIndex:id_UNIQUE"`

	// SQL: UNIQUE INDEX function_name_UNIQUE
	FunctionName        string `gorm:"column:function_name;type:varchar(45);not null;uniqueIndex:function_name_UNIQUE"`
	FunctionDescription string `gorm:"column:function_description;type:mediumtext"`

	// SQL: INDEX fk_AgentFunction_Agent1_idx
	AgentID int32 `gorm:"column:agent_id;not null;index:fk_AgentFunction_Agent1_idx"`

	Agent Agent `gorm:"foreignKey:AgentID"`
}

// Tablo: user_agent
type UserAgent struct {
	// SQL: INDEX fk_User_has_Agent_User1_idx
	// VE UNIQUE INDEX idx_user_agent_composite
	UserID int32 `gorm:"primaryKey;column:user_id;not null;index:fk_User_has_Agent_User1_idx;index:idx_user_agent_composite,unique"`

	// SQL: INDEX fk_User_has_Agent_Agent1_idx
	// VE UNIQUE INDEX idx_user_agent_composite
	AgentID int32 `gorm:"primaryKey;column:agent_id;not null;index:fk_User_has_Agent_Agent1_idx;index:idx_user_agent_composite,unique"`

	ConfigJSON    []byte    `gorm:"column:config_json;type:json"`
	IsActive      int       `gorm:"column:is_active;type:tinyint;not null;default:1"`
	ActivatedAt   time.Time `gorm:"column:activated_at;type:datetime;default:CURRENT_TIMESTAMP"`
	DeactivatedAt time.Time `gorm:"column:deactivated_at;type:datetime"`

	User  User  `gorm:"foreignKey:UserID"`
	Agent Agent `gorm:"foreignKey:AgentID"`
}

// Tablo: message
type Message struct {
	ID            int32  `gorm:"primaryKey;column:id;autoIncrement;not null"`
	Role          string `gorm:"column:role;type:varchar(45);not null"`
	Content       string `gorm:"column:content;type:longtext"`
	ToolCallsJSON []byte `gorm:"column:tool_calls;type:json"`

	// SQL: INDEX idx_message_created_at
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;index:idx_message_created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime"`

	// SQL: INDEX idx_message_deleted_at
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index:idx_message_deleted_at"`

	// SQL: INDEX idx_message_user_id
	UserID int32 `gorm:"column:user_id;not null;index:idx_message_user_id"`

	User User `gorm:"foreignKey:UserID"`
}
