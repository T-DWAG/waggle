package model

import (
	"time"

	"github.com/google/uuid"
)

type AgentStatus string

var (
	Draft     AgentStatus = "draft"
	Published AgentStatus = "published"
	Archived  AgentStatus = "archived"
)

type AgentVisibility string

var (
	Private  AgentVisibility = "private"
	Public   AgentVisibility = "public"
	LinkOnly AgentVisibility = "link_only"
)

// Agent 定义了智能代理的模型
type Agent struct {
	BaseModel
	// CreatorID 创建者ID，标识该agent的创建者
	CreatorID uuid.UUID `json:"creatorId" gorm:"column:creator_id;type:uuid;not null"`
	// Name agent名称
	Name string `json:"name" gorm:"column:name;type:varchar(255);not null"`
	// Description 描述信息
	Description string `json:"description" gorm:"column:description;type:text"`
	// Icon 图标URL或路径
	Icon string `json:"icon" gorm:"column:icon;type:varchar(512)"`
	// SystemPrompt 系统提示词，用于指导AI行为
	SystemPrompt string `json:"systemPrompt" gorm:"column:system_prompt;type:text"`
	// ModelProvider 模型提供商（例如openai）
	ModelProvider string `json:"modelProvider" gorm:"column:model_provider;type:varchar(50);not null;default:'openai'"`
	// ModelName 使用的具体模型名称
	ModelName string `json:"modelName" gorm:"column:model_name;type:varchar(100);not null"`
	// ModelParameters 模型参数配置
	ModelParameters JSON `json:"modelParameters" gorm:"column:model_parameters;type:jsonb"`
	// OpeningDialogue 开场白对话内容
	OpeningDialogue string `json:"openingDialogue" gorm:"column:opening_dialogue;type:text"`
	// SuggestedQuestions 建议问题列表
	SuggestedQuestions JSON `json:"suggestedQuestions" gorm:"column:suggested_questions;type:jsonb"`
	// Version 版本号
	Version uint `json:"version" gorm:"column:version;type:int;not null;default:1"`
	// Status 状态（草稿、发布、归档）
	Status AgentStatus `json:"status" gorm:"column:status;type:varchar(20);not null;default:'draft'"`
	// Visibility 可见性（私有、公开、仅链接）
	Visibility AgentVisibility `json:"visibility" gorm:"column:visibility;type:varchar(20);not null;default:'private'"`
	// InvocationCount 调用次数统计
	InvocationCount uint64 `json:"invocationCount" gorm:"column:invocation_count;type:bigint;not null;default:0"`
	// PublishedAt 发布时间戳
	PublishedAt *time.Time `json:"publishedAt" gorm:"column:published_at;type:timestamptz"`

	Tools []Tool `json:"tools,omitempty" gorm:"many2many:agent_tools;"`
}

// TableName 返回表名
func (Agent) TableName() string {
	return "agents"
}

// QuestionItem 单个建议问题项
type QuestionItem struct {
	Text  string `json:"text"`
	Label string `json:"label,omitempty"`
}
