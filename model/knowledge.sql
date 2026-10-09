-- 知识库：知识库 / 文档 / 切片 / 智能体关联。已有数据库执行本文件即可，不需要重建 agents、tools。
-- 全部 IF NOT EXISTS，重复执行无副作用。

-- ==========================================
-- Table: knowledge_bases
-- ==========================================
CREATE TABLE IF NOT EXISTS knowledge_bases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    creator_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,

    chat_model_name VARCHAR(255),
    chat_model_provider VARCHAR(50),
    embedding_model_name VARCHAR(255),
    embedding_model_provider VARCHAR(50),
    -- 建库时用探针实测写入；ES 索引 mapping 依赖它，创建后不能改。
    embedding_dimension INTEGER NOT NULL,

    storage_type VARCHAR(50) NOT NULL DEFAULT 'es',
    storage_config JSONB,
    -- kb_<uuid 去掉横线>
    index_name VARCHAR(100) NOT NULL,

    document_count INTEGER NOT NULL DEFAULT 0,
    chunk_count INTEGER NOT NULL DEFAULT 0,
    tags JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'active'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_kb_index_name ON knowledge_bases(index_name);
CREATE INDEX IF NOT EXISTS idx_kb_creator_id ON knowledge_bases(creator_id);
CREATE INDEX IF NOT EXISTS idx_kb_name ON knowledge_bases(name);
CREATE INDEX IF NOT EXISTS idx_kb_deleted_at ON knowledge_bases(deleted_at);
CREATE INDEX IF NOT EXISTS idx_kb_tags ON knowledge_bases USING GIN (tags);

-- ==========================================
-- Table: documents
-- ==========================================
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    kb_id UUID NOT NULL,
    creator_id UUID NOT NULL,

    name VARCHAR(255) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    token_count INTEGER NOT NULL DEFAULT 0,

    -- 对象存储里的原文 key；重索引靠它重新拉原文。
    storage_key VARCHAR(512) NOT NULL DEFAULT '',
    file_hash VARCHAR(64),

    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message TEXT,

    meta_info JSONB,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_docs_kb_id ON documents(kb_id);
CREATE INDEX IF NOT EXISTS idx_docs_creator_id ON documents(creator_id);
CREATE INDEX IF NOT EXISTS idx_docs_file_hash ON documents(file_hash);
CREATE INDEX IF NOT EXISTS idx_docs_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_docs_deleted_at ON documents(deleted_at);
CREATE INDEX IF NOT EXISTS idx_docs_meta_info ON documents USING GIN (meta_info);

-- ==========================================
-- Table: document_chunks
-- ==========================================
CREATE TABLE IF NOT EXISTS document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    document_id UUID NOT NULL,
    kb_id UUID NOT NULL,

    es_id VARCHAR(100),

    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    token_count INTEGER NOT NULL DEFAULT 0,

    meta_info JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
);

CREATE INDEX IF NOT EXISTS idx_chunks_document_id ON document_chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_chunks_kb_id ON document_chunks(kb_id);
CREATE INDEX IF NOT EXISTS idx_chunks_es_id ON document_chunks(es_id);
CREATE INDEX IF NOT EXISTS idx_chunks_deleted_at ON document_chunks(deleted_at);

-- ==========================================
-- Table: agent_knowledge_bases
-- status 与 agent_tools 同构：会话层按 enabled 过滤，可以只解绑不删库。
-- ==========================================
CREATE TABLE IF NOT EXISTS agent_knowledge_bases (
    agent_id UUID NOT NULL,
    knowledge_base_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'enabled',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (agent_id, knowledge_base_id),
    CONSTRAINT fk_akb_agent FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    CONSTRAINT fk_akb_kb FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_akb_kb_id ON agent_knowledge_bases(knowledge_base_id);

-- 08 父子分段：切片模式。已有库默认 flat，行为与 07 完全一致。
ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS chunk_mode VARCHAR(20) NOT NULL DEFAULT 'flat';
