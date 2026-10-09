package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// StringArrayJSON 让 []string 能直接读写 PostgreSQL 的 jsonb 列。
// PG 没有 text[] 以外的数组友好映射，用 jsonb 可以跨库移植，也方便 GIN 索引。
type StringArrayJSON []string

func (s StringArrayJSON) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	return json.Marshal(s)
}

func (s *StringArrayJSON) Scan(value interface{}) error {
	if value == nil {
		*s = StringArrayJSON{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan StringArrayJSON")
	}
	if len(bytes) == 0 {
		*s = StringArrayJSON{}
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// StorageType 知识库底层检索存储的类型，第一版只有 Elasticsearch。
type StorageType string

const (
	StorageTypeElasticSearch StorageType = "es"
)

// ChunkMode 切片模式：flat 每片一条；parent_child 子块进向量库、父块存 PG 回填。
type ChunkMode string

const (
	ChunkModeFlat        ChunkMode = "flat"
	ChunkModeParentChild ChunkMode = "parent_child"
)

type KnowledgeBaseStatus string

const (
	KnowledgeBaseStatusActive   KnowledgeBaseStatus = "active"
	KnowledgeBaseStatusDisabled KnowledgeBaseStatus = "disabled"
)

// KnowledgeBase 一个知识库 = 一个 ES 索引 + 一份文档清单。
// EmbeddingDimension 在建库时用探针实测写入，索引 mapping 依赖它，创建后不可修改。
type KnowledgeBase struct {
	BaseModel
	CreatorID              uuid.UUID           `json:"creatorId" gorm:"column:creator_id;type:uuid;not null;index"`
	Name                   string              `json:"name" gorm:"column:name;type:varchar(255);not null;index"`
	Description            string              `json:"description" gorm:"column:description;type:text"`
	ChatModelName          string              `json:"chatModelName" gorm:"column:chat_model_name;type:varchar(255)"`
	ChatModelProvider      string              `json:"chatModelProvider" gorm:"column:chat_model_provider;type:varchar(50)"`
	EmbeddingModelName     string              `json:"embeddingModelName" gorm:"column:embedding_model_name;type:varchar(255)"`
	EmbeddingModelProvider string              `json:"embeddingModelProvider" gorm:"column:embedding_model_provider;type:varchar(50)"`
	EmbeddingDimension     int                 `json:"embeddingDimension" gorm:"column:embedding_dimension;type:integer;not null"`
	StorageType            StorageType         `json:"storageType" gorm:"column:storage_type;type:varchar(50);not null;default:'es'"`
	StorageConfig          JSON                `json:"storageConfig" gorm:"column:storage_config;type:jsonb"`
	ChunkMode              ChunkMode           `json:"chunkMode" gorm:"column:chunk_mode;type:varchar(20);not null;default:'flat'"`
	IndexName              string              `json:"indexName" gorm:"column:index_name;type:varchar(100);not null;unique"`
	DocumentCount          uint                `json:"documentCount" gorm:"column:document_count;type:integer;not null;default:0"`
	ChunkCount             uint                `json:"chunkCount" gorm:"column:chunk_count;type:integer;not null;default:0"`
	Tags                   StringArrayJSON     `json:"tags" gorm:"column:tags;type:jsonb"`
	Status                 KnowledgeBaseStatus `json:"status" gorm:"column:status;type:varchar(20);not null;default:'active'"`

	Agents []Agent `json:"agents,omitempty" gorm:"many2many:agent_knowledge_bases;"`
}

func (*KnowledgeBase) TableName() string {
	return "knowledge_bases"
}

type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "pending"
	DocumentStatusProcessing DocumentStatus = "processing"
	DocumentStatusCompleted  DocumentStatus = "completed"
	DocumentStatusFailed     DocumentStatus = "failed"
)

// Document 原始文档的元数据。StorageKey 指向对象存储里的原文，
// 重索引时用它重新拉原文，切片参数改了也能真正重切。
type Document struct {
	BaseModel
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId" gorm:"column:kb_id;type:uuid;not null;index"`
	CreatorID       uuid.UUID `json:"creatorId" gorm:"column:creator_id;type:uuid;not null;index"`

	Name       string `json:"name" gorm:"column:name;type:varchar(255);not null"`
	FileType   string `json:"fileType" gorm:"column:file_type;type:varchar(50);not null"`
	Size       int64  `json:"size" gorm:"column:size;type:bigint;not null;default:0"`
	TokenCount int    `json:"tokenCount" gorm:"column:token_count;type:integer;not null;default:0"`

	StorageKey string `json:"storageKey" gorm:"column:storage_key;type:varchar(512);not null;default:''"`
	FileHash   string `json:"fileHash" gorm:"column:file_hash;type:varchar(64);index"`

	Status       DocumentStatus `json:"status" gorm:"column:status;type:varchar(20);not null;default:'pending';index"`
	ErrorMessage string         `json:"errorMessage" gorm:"column:error_message;type:text"`

	MetaInfo JSON `json:"metaInfo" gorm:"column:meta_info;type:jsonb"`
	Enabled  bool `json:"enabled" gorm:"column:enabled;type:boolean;not null;default:true"`
}

func (*Document) TableName() string {
	return "documents"
}

type ChunkStatus string

const (
	ChunkStatusPending  ChunkStatus = "pending"
	ChunkStatusEmbedded ChunkStatus = "embedded"
	ChunkStatusDeleted  ChunkStatus = "deleted"
	ChunkStatusDisabled ChunkStatus = "disabled"
)

// DocumentChunk PostgreSQL 侧的切片权威副本；ES 里那份只用于检索。
type DocumentChunk struct {
	BaseModel
	DocumentID      uuid.UUID `json:"documentId" gorm:"column:document_id;type:uuid;not null;index"`
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId" gorm:"column:kb_id;type:uuid;not null;index"`

	ElasticSearchID string `json:"esId" gorm:"column:es_id;type:varchar(100);index"`

	ChunkIndex int    `json:"chunkIndex" gorm:"column:chunk_index;type:integer;not null"`
	Content    string `json:"content" gorm:"column:content;type:text;not null"`
	TokenCount int    `json:"tokenCount" gorm:"column:token_count;type:integer;not null;default:0"`

	MetaInfo JSON        `json:"metaInfo" gorm:"column:meta_info;type:jsonb"`
	Status   ChunkStatus `json:"status" gorm:"column:status;type:varchar(20);not null;default:'pending'"`
}

func (*DocumentChunk) TableName() string {
	return "document_chunks"
}

// AgentKnowledgeBase 智能体与知识库的关联，带状态以便会话层按 enabled 过滤。
type AgentKnowledgeBase struct {
	AgentID         uuid.UUID `json:"agentId" gorm:"column:agent_id;type:uuid;primaryKey"`
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId" gorm:"column:knowledge_base_id;type:uuid;primaryKey;index"`
	Status          string    `json:"status" gorm:"column:status;size:50;default:'enabled'"`
	CreatedAt       time.Time `json:"createdAt" gorm:"column:created_at"`
}

func (AgentKnowledgeBase) TableName() string {
	return "agent_knowledge_bases"
}
