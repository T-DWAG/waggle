package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

var (
	OllamaProvider = "ollama"
	OpenAIProvider = "openai"
	QwenProvider   = "qwen"
)

// LLMStatus 定义了模型状态
type LLMStatus string

var (
	LLMStatusActive   LLMStatus = "active"
	LLMStatusInactive LLMStatus = "inactive"
)

// LLMType 定义了模型类型
type LLMType string

var (
	LLMTypeChat      LLMType = "chat"      // 对话模型
	LLMTypeEmbedding LLMType = "embedding" // 向量模型
	LLMTypeVision    LLMType = "vision"    // 图像模型
)

// ProviderConfig 定义了厂商配置：名称、API地址、API秘钥、状态
type ProviderConfig struct {
	BaseModel
	UserID      uuid.UUID `json:"userId" gorm:"column:user_id;type:uuid;not null;index"`         // 用户ID
	Name        string    `json:"name" gorm:"column:name;type:varchar(255);not null"`            // 提供商名称
	Provider    string    `json:"provider" gorm:"column:provider;type:varchar(50);not null"`     // 提供商标识
	Description string    `json:"description" gorm:"column:description;type:text"`               // 描述
	APIKey      string    `json:"apiKey" gorm:"column:api_key;type:varchar(255)"`                // API密钥
	APIBase     string    `json:"apiBase" gorm:"column:api_base;type:varchar(255)"`              // API地址
	Status      LLMStatus `json:"status" gorm:"column:status;type:varchar(20);default:'active'"` // 状态
}

// TableName 返回表名
func (ProviderConfig) TableName() string {
	return "provider_configs"
}

// LLM 定义了用户自定义的AI大语言模型
type LLM struct {
	BaseModel
	UserID           uuid.UUID      `json:"userId" gorm:"column:user_id;type:uuid;not null;index"`              // 用户ID
	Name             string         `json:"name" gorm:"column:name;type:varchar(255);not null"`                 // 模型名称
	Description      string         `json:"description" gorm:"column:description;type:text"`                    // 描述
	ProviderConfigID uuid.UUID      `json:"providerConfigId" gorm:"column:provider_config_id;type:uuid"`        // 关联的厂商配置ID
	ProviderConfig   ProviderConfig `json:"providerConfig" gorm:"foreignKey:ProviderConfigID"`                  // 关联的厂商配置
	ModelName        string         `json:"modelName" gorm:"column:model_name;type:varchar(255);not null"`      // 模型标识
	ModelType        LLMType        `json:"modelType" gorm:"column:model_type;type:varchar(20);default:'chat'"` // 模型类型
	Config           LLMConfig      `json:"config" gorm:"column:config;type:jsonb"`                             // 其他关键配置
	Status           LLMStatus      `json:"status" gorm:"column:status;type:varchar(20);default:'active'"`      // 状态
}

// TableName 返回表名
func (*LLM) TableName() string {
	return "llms"
}

// LLMConfig 大模型的关键配置
type LLMConfig struct {
	MaxTokens   int     `json:"maxTokens"`
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"topP"`
}

func (c LLMConfig) Value() (driver.Value, error) {
	if c.MaxTokens == 0 && c.Temperature == 0 && c.TopP == 0 {
		return nil, nil
	}
	return json.Marshal(c)
}

func (c *LLMConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("无法扫描为 []byte 类型")
	}
	if len(bytes) == 0 {
		return nil
	}
	return json.Unmarshal(bytes, c)
}
