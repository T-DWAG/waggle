package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type ToolType string

const (
	McpToolType    ToolType = "mcp"
	SystemToolType ToolType = "system"
)

const (
	Enabled  = "enabled"
	Disabled = "disabled"
)

// Tool 定义了工具的模型。
type Tool struct {
	BaseModel
	CreatorID        uuid.UUID        `json:"creatorId" gorm:"column:creator_id;type:uuid;index;not null"`
	Name             string           `json:"name" gorm:"column:name;size:255;not null;index"`
	Description      string           `json:"description" gorm:"column:description;type:text"`
	ToolType         ToolType         `json:"toolType" gorm:"column:tool_type;size:50;not null"`
	IsEnable         bool             `json:"isEnable" gorm:"column:is_enable;default:true"`
	ParametersSchema ParametersSchema `json:"parametersSchema" gorm:"column:parameters_schema;type:jsonb"`
	McpConfig        *McpConfig       `json:"mcpConfig,omitempty" gorm:"column:mcp_config;type:jsonb"`
	Agents           []Agent          `json:"agents,omitempty" gorm:"many2many:agent_tools;"`
}

func (Tool) TableName() string {
	return "tools"
}

// AgentTool 是智能体与工具的关联，允许保存关联状态。
type AgentTool struct {
	AgentID   uuid.UUID `json:"agentId" gorm:"column:agent_id;type:uuid;primaryKey"`
	ToolID    uuid.UUID `json:"toolId" gorm:"column:tool_id;type:uuid;primaryKey;index"`
	Status    string    `json:"status" gorm:"column:status;size:50;default:'enabled'"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at"`
}

func (AgentTool) TableName() string {
	return "agent_tools"
}

type McpConfig struct {
	Type                   string `json:"type,omitempty"`
	Url                    string `json:"url,omitempty"`
	AuthenticationRequired bool   `json:"authenticationRequired,omitempty"`
	CredentialType         string `json:"credentialType,omitempty"`
}

func (c McpConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *McpConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}
	return json.Unmarshal(bytes, c)
}

type ParametersSchema map[string]*schema.ParameterInfo

func (s ParametersSchema) Value() (driver.Value, error) {
	if s == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(s)
}

func (s *ParametersSchema) Scan(value interface{}) error {
	if value == nil {
		*s = ParametersSchema{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}
	return json.Unmarshal(bytes, s)
}
