package tools

import (
	"model"

	"github.com/google/uuid"
)

type CreateToolRequest struct {
	Name        string           `json:"name" binding:"required,max=255"`
	Description string           `json:"description"`
	ToolType    model.ToolType   `json:"toolType"`
	IsEnable    *bool            `json:"isEnable"`
	McpConfig   *model.McpConfig `json:"mcpConfig"`
}

type UpdateToolRequest struct {
	Name             *string                 `json:"name"`
	Description      *string                 `json:"description"`
	ToolType         model.ToolType          `json:"toolType"`
	ParametersSchema *model.ParametersSchema `json:"parametersSchema"`
	McpConfig        *model.McpConfig        `json:"mcpConfig"`
	IsEnable         *bool                   `json:"isEnable"`
}

type ToolResponse struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Type             string                 `json:"type"`
	IsEnable         bool                   `json:"isEnable"`
	ParametersSchema model.ParametersSchema `json:"parametersSchema"`
	Config           model.JSON             `json:"config"`
	CreatedAt        string                 `json:"createdAt"`
	UpdatedAt        string                 `json:"updatedAt"`
}

type ToolListRequest struct {
	Name     string `json:"name" form:"name"`
	Type     string `json:"type" form:"type"`
	Page     int    `json:"page" form:"page"`
	PageSize int    `json:"pageSize" form:"pageSize"`
}

type toolFilter struct {
	CreatorID uuid.UUID
	Name      string
	ToolType  string
	Limit     int
	Offset    int
}

type TestToolRequest struct {
	Params map[string]any `json:"params"`
}

type TestToolResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}
