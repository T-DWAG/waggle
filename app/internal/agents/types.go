package agents

import (
	"model"

	"github.com/google/uuid"
)

const (
	defaultPageSize = 10
	maxPageSize     = 100
)

// AgentFilter is used only by the repository layer.
type AgentFilter struct {
	Name   string
	Status model.AgentStatus
	Limit  int
	Offset int
}

// ProviderConfigFilter is used only by the repository layer.
type ProviderConfigFilter struct {
	UserID   uuid.UUID
	Name     string
	Provider string
	Status   model.LLMStatus
	Limit    int
	Offset   int
}

// LLMFilter is used only by the repository layer.
type LLMFilter struct {
	UserID           uuid.UUID
	Name             string
	ProviderConfigID *uuid.UUID
	ModelType        model.LLMType
	Status           model.LLMStatus
	Limit            int
	Offset           int
}

// SearchRequest is the request body for POST /api/v1/agents/list.
type SearchRequest struct {
	Name     string            `json:"name"`
	Status   model.AgentStatus `json:"status"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

type CreateAgentRequest struct {
	Name        string            `json:"name" binding:"required,max=255"`
	Description string            `json:"description"`
	Status      model.AgentStatus `json:"status"`
}

// Pointer fields distinguish an omitted field from an explicit empty value.
type UpdateAgentRequest struct {
	ID                 uuid.UUID              `json:"id" binding:"required"`
	Name               *string                `json:"name"`
	Description        *string                `json:"description"`
	Icon               *string                `json:"icon"`
	Status             *model.AgentStatus     `json:"status"`
	Visibility         *model.AgentVisibility `json:"visibility"`
	SystemPrompt       *string                `json:"systemPrompt"`
	ModelName          *string                `json:"modelName"`
	ModelProvider      *string                `json:"modelProvider"`
	ModelParameters    *model.JSON            `json:"modelParameters"`
	OpeningDialogue    *string                `json:"openingDialogue"`
	SuggestedQuestions *model.JSON            `json:"suggestedQuestions"`
}

type ChatRequest struct {
	AgentID   uuid.UUID  `json:"agentId" binding:"required"`
	Message   string     `json:"message" binding:"required"`
	SessionID *uuid.UUID `json:"sessionId,omitempty"`
}

type ListAgentResponse struct {
	Agents []*model.Agent `json:"agents"`
	Total  int64          `json:"total"`
}

// ProviderConfigListQuery and LLMListQuery are GET query parameters.
type ProviderConfigListQuery struct {
	Name     string          `form:"name"`
	Provider string          `form:"provider"`
	Status   model.LLMStatus `form:"status"`
	Page     int             `form:"page"`
	PageSize int             `form:"pageSize"`
}

type CreateProviderConfigRequest struct {
	Name        string          `json:"name" binding:"required,max=255"`
	Provider    string          `json:"provider" binding:"required,max=50"`
	Description string          `json:"description"`
	APIKey      string          `json:"apiKey"`
	APIBase     string          `json:"apiBase"`
	Status      model.LLMStatus `json:"status"`
}

type UpdateProviderConfigRequest struct {
	Name        *string          `json:"name"`
	Description *string          `json:"description"`
	APIKey      *string          `json:"apiKey"`
	APIBase     *string          `json:"apiBase"`
	Status      *model.LLMStatus `json:"status"`
}

// ProviderConfigResponse never exposes the stored API key.
type ProviderConfigResponse struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Provider    string          `json:"provider"`
	Description string          `json:"description"`
	APIKey      string          `json:"apiKey"`
	HasAPIKey   bool            `json:"hasApiKey"`
	APIBase     string          `json:"apiBase"`
	Status      model.LLMStatus `json:"status"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

type ListProviderConfigsResponse struct {
	ProviderConfigs []*ProviderConfigResponse `json:"providerConfigs"`
	Total           int64                     `json:"total"`
}

type CreateProviderConfigResponse struct {
	ID uuid.UUID `json:"id"`
}

type SuccessResponse struct {
	Success bool `json:"success"`
}

type LLMListQuery struct {
	Name             string          `form:"name"`
	ProviderConfigID *uuid.UUID      `form:"providerConfigId"`
	ModelType        model.LLMType   `form:"modelType"`
	Status           model.LLMStatus `form:"status"`
	Page             int             `form:"page"`
	PageSize         int             `form:"pageSize"`
}

type CreateLLMRequest struct {
	Name             string          `json:"name" binding:"required,max=255"`
	Description      string          `json:"description"`
	ProviderConfigID uuid.UUID       `json:"providerConfigId" binding:"required"`
	ModelName        string          `json:"modelName" binding:"required,max=255"`
	ModelType        model.LLMType   `json:"modelType"`
	Config           model.LLMConfig `json:"config"`
	Status           model.LLMStatus `json:"status"`
}

type UpdateLLMRequest struct {
	Name             *string          `json:"name"`
	Description      *string          `json:"description"`
	ProviderConfigID *uuid.UUID       `json:"providerConfigId"`
	ModelName        *string          `json:"modelName"`
	ModelType        *model.LLMType   `json:"modelType"`
	Config           *model.LLMConfig `json:"config"`
	Status           *model.LLMStatus `json:"status"`
}

type LLMResponse struct {
	ID               uuid.UUID       `json:"id"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	ProviderConfigID uuid.UUID       `json:"providerConfigId"`
	ModelName        string          `json:"modelName"`
	ModelType        model.LLMType   `json:"modelType"`
	Config           model.LLMConfig `json:"config"`
	Status           model.LLMStatus `json:"status"`
	CreatedAt        string          `json:"createdAt"`
	UpdatedAt        string          `json:"updatedAt"`
}

type ListLLMsResponse struct {
	LLMs  []*LLMResponse `json:"llms"`
	Total int64          `json:"total"`
}

type CreateLLMResponse struct {
	ID uuid.UUID `json:"id"`
}

type GetLLMResponse struct {
	LLM *LLMResponse `json:"llm"`
}
