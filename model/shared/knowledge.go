package shared

import "github.com/google/uuid"

// KnowledgeSearchParams 跨模块检索事件入参（agents → knowledge）。
// agents 只知道「这个智能体绑了哪些库」，怎么向量化、查哪个索引由 knowledge 模块决定。
type KnowledgeSearchParams struct {
	UserID           uuid.UUID   `json:"userId"`
	KnowledgeBaseIDs []uuid.UUID `json:"knowledgeBaseIds"`
	Query            string      `json:"query"`
	TopK             int         `json:"topK"`
}
