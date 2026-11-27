-- +goose Up
CREATE TABLE warning_notification_rules (
 id SERIAL PRIMARY KEY,
 admin_id INTEGER NOT NULL,
 workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 name VARCHAR(20) NOT NULL,
 enabled BOOLEAN NOT NULL DEFAULT TRUE,
 config_json JSONB NOT NULL,
 next_check_at BIGINT NOT NULL DEFAULT 0,
 next_notice_at BIGINT NOT NULL DEFAULT 0,
 config_revision BIGINT NOT NULL DEFAULT 1,
 lease_token TEXT NOT NULL DEFAULT '',
 lease_until BIGINT NOT NULL DEFAULT 0,
 UNIQUE(admin_id,workspace_id,id)
);
CREATE INDEX warning_rules_due ON warning_notification_rules(next_check_at) WHERE enabled;
CREATE TABLE warning_notification_deliveries (
 id BIGSERIAL PRIMARY KEY,
 admin_id INTEGER NOT NULL,
 workspace_id INTEGER NOT NULL,
 rule_id INTEGER NOT NULL,
 config_json JSONB NOT NULL,
 recipients JSONB NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt_at BIGINT NOT NULL DEFAULT 0,
 sent_at BIGINT NOT NULL DEFAULT 0,
 lease_token TEXT NOT NULL DEFAULT '',
 FOREIGN KEY(admin_id,workspace_id,rule_id) REFERENCES warning_notification_rules(admin_id,workspace_id,id) ON DELETE CASCADE
);
CREATE INDEX warning_deliveries_due ON warning_notification_deliveries(next_attempt_at) WHERE sent_at=0 AND attempts<3;
CREATE INDEX warning_requirements_window ON requirements(admin_id,workspace_id,template_id,created_at,id) WHERE deleted_at=0;
INSERT INTO role_permissions(admin_id,role_id,permission_key,created_at,updated_at)
SELECT r.admin_id,r.id,p.permission_key,EXTRACT(EPOCH FROM NOW())::BIGINT,EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r CROSS JOIN (VALUES ('warning_notification:view'),('warning_notification:manage')) p(permission_key)
WHERE r.scope='project' AND r.is_system=1 AND r.name LIKE '%perm_role_project_admin%'
ON CONFLICT(admin_id,role_id,permission_key) DO NOTHING;

-- +goose Down
DROP INDEX warning_requirements_window;
DROP TABLE warning_notification_deliveries;
DROP TABLE warning_notification_rules;
-- 不撤销权限授权，避免删除迁移后用户主动配置的角色权限。
