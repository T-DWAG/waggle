-- 1. agents
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    creator_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    icon VARCHAR(512),

    system_prompt TEXT,

    model_provider VARCHAR(50) NOT NULL DEFAULT 'openai',
    model_name VARCHAR(100) NOT NULL,

    model_parameters JSONB,

    opening_dialogue TEXT,
    suggested_questions JSONB,

    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 0),

    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    visibility VARCHAR(20) NOT NULL DEFAULT 'private',

    invocation_count BIGINT NOT NULL DEFAULT 0 CHECK (invocation_count >= 0),

    published_at TIMESTAMPTZ
);

CREATE INDEX idx_agents_deleted_at ON agents (deleted_at);
CREATE INDEX idx_agents_creator_id ON agents (creator_id);
CREATE INDEX idx_agents_status ON agents (status);

-- 2. provider_configs
CREATE TABLE provider_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    description TEXT,
    api_key VARCHAR(255),
    api_base VARCHAR(255),
    status VARCHAR(20) DEFAULT 'active'
);

CREATE INDEX idx_provider_configs_deleted_at ON provider_configs (deleted_at);
CREATE INDEX idx_provider_configs_user_id ON provider_configs (user_id);
CREATE INDEX idx_provider_configs_provider ON provider_configs (provider);

COMMENT ON TABLE provider_configs IS '大模型厂商配置表';
COMMENT ON COLUMN provider_configs.name IS '配置名称(如: 我的OpenAI)';
COMMENT ON COLUMN provider_configs.provider IS '厂商标识(openai, ollama, qwen)';
COMMENT ON COLUMN provider_configs.api_key IS 'API密钥';
COMMENT ON COLUMN provider_configs.api_base IS 'API地址(Endpoint)';
COMMENT ON COLUMN provider_configs.status IS '状态: active, inactive';

-- 3. llms
CREATE TABLE llms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,

    provider_config_id UUID,

    model_name VARCHAR(255) NOT NULL,
    model_type VARCHAR(20) DEFAULT 'chat',
    config JSONB,
    status VARCHAR(20) DEFAULT 'active'
);

CREATE INDEX idx_llms_deleted_at ON llms (deleted_at);
CREATE INDEX idx_llms_user_id ON llms (user_id);
CREATE INDEX idx_llms_provider_config_id ON llms (provider_config_id);

COMMENT ON TABLE llms IS '自定义大语言模型表';
COMMENT ON COLUMN llms.model_name IS '实际模型标识(如 gpt-4-turbo)';
COMMENT ON COLUMN llms.model_type IS '模型类型: chat, embedding, vision';
COMMENT ON COLUMN llms.config IS '模型参数配置(JSONB): maxTokens, temperature等';
COMMENT ON COLUMN llms.status IS '状态: active, inactive';
