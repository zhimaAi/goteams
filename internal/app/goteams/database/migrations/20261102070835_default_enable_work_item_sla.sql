-- +goose Up
-- +goose StatementBegin
ALTER TABLE workspace_sla_settings ALTER COLUMN enabled SET DEFAULT TRUE;
-- 新默认仅覆盖尚未启用的空间，保留启用后主动关闭的状态及暂停区间。
INSERT INTO workspace_sla_settings (admin_id,workspace_id,enabled,timezone,activated_at,created_at,updated_at)
SELECT admin_id,id,TRUE,'Asia/Shanghai',EXTRACT(EPOCH FROM NOW())::BIGINT,EXTRACT(EPOCH FROM NOW())::BIGINT,EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces WHERE workspace_type='issue'
ON CONFLICT (admin_id,workspace_id) DO NOTHING;
UPDATE workspace_sla_settings s SET enabled=TRUE,timezone=CASE WHEN s.timezone='' THEN 'Asia/Shanghai' ELSE s.timezone END,
activated_at=EXTRACT(EPOCH FROM NOW())::BIGINT,config_revision=config_revision+1,clock_revision=clock_revision+1,updated_at=EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w WHERE s.admin_id=w.admin_id AND s.workspace_id=w.id AND w.workspace_type='issue' AND s.activated_at=0;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE workspace_sla_settings ALTER COLUMN enabled SET DEFAULT FALSE;
-- 无法区分迁移默认值与后续用户设置，不回退开关及首次启用事实，避免恢复旧机制或误改用户配置。
