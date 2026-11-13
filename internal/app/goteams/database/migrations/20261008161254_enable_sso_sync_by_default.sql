-- +goose Up
-- +goose StatementBegin

ALTER TABLE sso_sync_configs ALTER COLUMN enabled SET DEFAULT 1;

-- 旧关闭状态统一转为开启；自动周期为 none 的租户仍只支持手动同步。
UPDATE sso_sync_configs SET enabled=1, updated_at=EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE enabled=0;

-- 仅恢复已配置好登录平台和同步凭证的旧任务，避免旧平台切换留下的空 SCIM 配置触发失败任务。
UPDATE scheduled_tasks AS task
SET enabled=1, updated_at=EXTRACT(EPOCH FROM NOW())::BIGINT
FROM sso_sync_configs AS sync
JOIN sso_configs AS login ON login.admin_id=sync.admin_id
WHERE task.task_key='sso_member_sync'
  AND task.scope_key='sso_member_sync:' || sync.admin_id::TEXT
  AND task.admin_id=sync.admin_id
  AND task.enabled=0
  AND sync.auto_sync_period<>'none'
  AND login.enabled=1
  AND (
    login.provider IN ('dingtalk', 'feishu')
    OR (login.provider IN ('oidc', 'oauth2') AND sync.scim_base_url<>'' AND sync.scim_token<>'')
  );

COMMENT ON COLUMN sso_sync_configs.enabled IS '成员同步可用状态，默认开启；none 周期不安排定时任务';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 无法区分迁移前手动关闭与原本开启的租户，保留已转为开启的数据。
ALTER TABLE sso_sync_configs ALTER COLUMN enabled SET DEFAULT 0;

-- +goose StatementEnd
