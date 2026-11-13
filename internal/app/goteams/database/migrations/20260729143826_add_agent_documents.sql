-- +goose Up
-- Agent 自定义文档：云端只保存用于客户端物化文档的稳定 ID 和占位符名称
CREATE TABLE agent_documents (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    agent_id INTEGER NOT NULL,
    client_document_id CHAR(32) NOT NULL,
    name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, agent_id, client_document_id),
    UNIQUE(admin_id, agent_id, name)
);

CREATE INDEX idx_agent_documents_order
    ON agent_documents(admin_id, agent_id, sort_order, id);

COMMENT ON TABLE agent_documents IS 'Agent 自定义文档占位符';
COMMENT ON COLUMN agent_documents.admin_id IS '租户 ID';
COMMENT ON COLUMN agent_documents.agent_id IS 'Agent ID';
COMMENT ON COLUMN agent_documents.client_document_id IS '客户端文档 ID，32 位随机十六进制字符串';
COMMENT ON COLUMN agent_documents.name IS '文档名称，同时作为提示词占位符名称';
COMMENT ON COLUMN agent_documents.sort_order IS '展示顺序';
COMMENT ON COLUMN agent_documents.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN agent_documents.updated_at IS '更新时间（Unix 秒）';

-- +goose Down
DROP TABLE IF EXISTS agent_documents;
