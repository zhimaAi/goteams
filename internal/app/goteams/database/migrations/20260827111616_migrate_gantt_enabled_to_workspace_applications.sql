-- +goose Up
-- +goose StatementBegin

-- 甘特图应用开关从 workspaces_settings.gantt_enabled 迁移到 workspace_applications 表
-- （应用市场开关统一使用 workspace_applications 方案，app_key=gantt）。

INSERT INTO workspace_applications (admin_id, workspace_id, app_key, enabled, enabled_by, enabled_at, updated_at)
SELECT admin_id, workspace_id, 'gantt', 1, 0,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces_settings
WHERE gantt_enabled = 1
ON CONFLICT (admin_id, workspace_id, app_key) DO NOTHING;

ALTER TABLE workspaces_settings DROP COLUMN IF EXISTS gantt_enabled;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE workspaces_settings ADD COLUMN IF NOT EXISTS gantt_enabled SMALLINT NOT NULL DEFAULT 0;

UPDATE workspaces_settings s
SET gantt_enabled = 1, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspace_applications wa
WHERE wa.admin_id = s.admin_id
  AND wa.workspace_id = s.workspace_id
  AND wa.app_key = 'gantt'
  AND wa.enabled = 1;

-- +goose StatementEnd
