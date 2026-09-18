package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CreatedAt time.Time      `json:"createdAt" gorm:"column:created_at;not null"`
	UpdatedAt time.Time      `json:"updatedAt" gorm:"column:updated_at;not null"`
	DeletedAt gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"column:deleted_at;index"`
}

// JSON type for PostgreSQL jsonb fields
type JSON map[string]interface{}

// NewJSON 将任意类型转换为JSON类型
func NewJSON(v interface{}) (JSON, error) {
	if v == nil {
		return make(JSON), nil
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}
	return JSON(result), nil
}

// Scan 实现 sql.Scanner 接口，用于从数据库读取 JSON 数据
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into JSON", value)
	}
	result := make(map[string]interface{})
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}
	*j = JSON(result)
	return nil
}

// Value 实现 driver.Valuer 接口，用于将 JSON 数据写入数据库
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return make(JSON), nil
	}
	return json.Marshal(j)
}

// ToModelParams 将JSON类型的ModelParameters转换为ModelsParams结构体
func (j JSON) ToModelParams() ModelsParams {
	params := ModelsParams{}
	if maxTokens, ok := j["maxTokens"].(float64); ok {
		params.MaxTokens = int(maxTokens)
	}
	if temperature, ok := j["temperature"].(float64); ok {
		params.Temperature = temperature
	}
	if topP, ok := j["topP"].(float64); ok {
		params.TopP = topP
	}
	if n, ok := j["n"].(float64); ok {
		params.N = int(n)
	}
	if stop, ok := j["stop"].([]any); ok {
		params.Stop = stop
	}
	if presencePenalty, ok := j["presencePenalty"].(float64); ok {
		params.PresencePenalty = presencePenalty
	}
	if frequencyPenalty, ok := j["frequencyPenalty"].(float64); ok {
		params.FrequencyPenalty = frequencyPenalty
	}
	return params
}

// ModelsParams 定义了模型参数的结构
type ModelsParams struct {
	// MaxTokens 最大生成长度（单位：Token），不包含输入的 Prompt 长度
	// 注意：(Input Tokens + MaxTokens) 不能超过模型的上下文窗口上限
	MaxTokens int `json:"maxTokens"`
	// Temperature 控制模型输出的随机性
	// 0.0 几乎确定性（代码生成）；1.0+ 增加多样性（创意写作）
	Temperature float64 `json:"temperature"`
	// TopP 核采样：只考虑累积概率达到 TopP 的 Token 集合
	// 最佳实践：一般只改 Temperature 或 TopP 其中之一，不要同时改
	TopP float64 `json:"topP"`
	// N 针对同一条提示词一次生成多少条独立回复
	N int `json:"n"`
	// Stop 停止词，生成到该序列立即终止，且结果不包含它
	Stop []any `json:"stop"`
	// PresencePenalty 话题新鲜度惩罚：基于 Token 是否出现过，值越大越愿意换话题
	PresencePenalty float64 `json:"presencePenalty"`
	// FrequencyPenalty 重复度惩罚：基于出现频次，值越大越排斥复读
	FrequencyPenalty float64 `json:"frequencyPenalty"`
}
