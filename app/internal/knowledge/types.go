package knowledge

import (
	"time"

	"model"

	"github.com/google/uuid"
)

type CreateKnowledgeBaseRequest struct {
	Name                   string   `json:"name" binding:"required,max=255"`
	Description            string   `json:"description"`
	EmbeddingModelProvider string   `json:"embeddingModelProvider" binding:"required,max=50"`
	EmbeddingModelName     string   `json:"embeddingModelName" binding:"required,max=255"`
	Tags                   []string `json:"tags"`
	ChunkMode              string   `json:"chunkMode"` // 空 = flat
}

// UpdateKnowledgeBaseRequest 只允许改展示信息。
// embedding 字段出现在请求里且与原值不同会返回 4009：索引维度建好后不能改。
type UpdateKnowledgeBaseRequest struct {
	Name                   *string   `json:"name"`
	Description            *string   `json:"description"`
	Tags                   *[]string `json:"tags"`
	Status                 *string   `json:"status"`
	EmbeddingModelProvider *string   `json:"embeddingModelProvider"`
	EmbeddingModelName     *string   `json:"embeddingModelName"`
	ChunkMode              *string   `json:"chunkMode"` // 仅用于拒绝修改
}

type ListKnowledgeBasesRequest struct {
	Name     string `json:"name"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type KnowledgeBaseResponse struct {
	ID                     uuid.UUID `json:"id"`
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	EmbeddingModelProvider string    `json:"embeddingModelProvider"`
	EmbeddingModelName     string    `json:"embeddingModelName"`
	EmbeddingDimension     int       `json:"embeddingDimension"`
	StorageType            string    `json:"storageType"`
	ChunkMode              string    `json:"chunkMode"`
	IndexName              string    `json:"indexName"`
	DocumentCount          uint      `json:"documentCount"`
	ChunkCount             uint      `json:"chunkCount"`
	TotalSize              int64     `json:"totalSize"`
	Tags                   []string  `json:"tags"`
	Status                 string    `json:"status"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type ListDocumentsRequest struct {
	Name     string `form:"name"`
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
}

type DocumentResponse struct {
	ID              uuid.UUID `json:"id"`
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId"`
	Name            string    `json:"name"`
	FileType        string    `json:"fileType"`
	Size            int64     `json:"size"`
	TokenCount      int       `json:"tokenCount"`
	ChunkCount      int64     `json:"chunkCount"`
	Status          string    `json:"status"`
	ErrorMessage    string    `json:"errorMessage,omitempty"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type UpdateDocumentRequest struct {
	Enabled *bool `json:"enabled"`
}

type ChunkResponse struct {
	ID         uuid.UUID  `json:"id"`
	ChunkIndex int        `json:"chunkIndex"`
	Content    string     `json:"content"`
	TokenCount int        `json:"tokenCount"`
	Section    string     `json:"section"`
	MetaInfo   model.JSON `json:"metaInfo"`
	Status     string     `json:"status"`
}

type SearchRequest struct {
	Query       string      `json:"query" binding:"required"`
	TopK        int         `json:"topK"`
	MinScore    *float64    `json:"minScore"`
	DocumentIDs []uuid.UUID `json:"documentIds"`
}

type SearchResult struct {
	ChunkID      string         `json:"chunkId"`
	DocumentID   string         `json:"documentId"`
	DocumentName string         `json:"documentName"`
	FileType     string         `json:"fileType"`
	Position     int            `json:"position"`
	Section      string         `json:"section"`
	Content      string         `json:"content"`
	Score        float64        `json:"score"`
	VectorScore  float64        `json:"vectorScore"`
	KeywordScore float64        `json:"keywordScore"`
	Metadata     map[string]any `json:"metadata"`
	Matched      string         `json:"matched,omitempty"` // parent_child：命中的子块正文
}

type SearchResponse struct {
	Query           string         `json:"query"`
	KnowledgeBaseID uuid.UUID      `json:"knowledgeBaseId"`
	Mode            string         `json:"mode"`
	Took            int64          `json:"took"`
	Total           int            `json:"total"`
	Results         []SearchResult `json:"results"`
}

type knowledgeFilter struct {
	CreatorID uuid.UUID
	Name      string
	Limit     int
	Offset    int
}

type documentFilter struct {
	KnowledgeBaseID uuid.UUID
	Name            string
	Status          string
	Limit           int
	Offset          int
}
