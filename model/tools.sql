-- 工具与智能体工具关联。已有数据库执行本文件即可，不需要重建 agents。
CREATE TABLE IF NOT EXISTS tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    creator_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    tool_type VARCHAR(50) NOT NULL,
    is_enable BOOLEAN NOT NULL DEFAULT TRUE,
    parameters_schema JSONB,
    mcp_config JSONB
);

CREATE INDEX IF NOT EXISTS idx_tools_deleted_at ON tools(deleted_at);
CREATE INDEX IF NOT EXISTS idx_tools_creator_id ON tools(creator_id);
CREATE INDEX IF NOT EXISTS idx_tools_name ON tools(name);
CREATE INDEX IF NOT EXISTS idx_tools_tool_type ON tools(tool_type);

CREATE TABLE IF NOT EXISTS agent_tools (
    agent_id UUID NOT NULL,
    tool_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'enabled',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (agent_id, tool_id),
    CONSTRAINT fk_agent_tools_agent FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    CONSTRAINT fk_agent_tools_tool FOREIGN KEY (tool_id) REFERENCES tools(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_tools_tool_id ON agent_tools(tool_id);
