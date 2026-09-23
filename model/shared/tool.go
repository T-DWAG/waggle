package shared

import "github.com/google/uuid"

// ToolsReq 是按 ID 批量查询工具的事件入参。
type ToolsReq struct {
	Ids []uuid.UUID `json:"ids"`
}
