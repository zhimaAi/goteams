-- +goose Up
-- +goose StatementBegin
CREATE TABLE enterprise_personalizations (
    admin_id INTEGER PRIMARY KEY,
    app_name VARCHAR(100) NOT NULL DEFAULT '',
    logo VARCHAR(1000) NOT NULL DEFAULT '',
    domain_prefix VARCHAR(63) NOT NULL DEFAULT '',
    login_slogan TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT enterprise_personalizations_domain_format CHECK (
        domain_prefix = '' OR domain_prefix ~ '^[a-z0-9][a-z0-9-]{5,61}[a-z0-9]$'
    )
);
COMMENT ON TABLE enterprise_personalizations IS '企业个性化设置，按租户隔离';
COMMENT ON COLUMN enterprise_personalizations.logo IS '应用图标存储路径，不保存临时签名地址';
COMMENT ON COLUMN enterprise_personalizations.login_slogan IS '登录页富文本标语，空值恢复默认';
CREATE UNIQUE INDEX enterprise_personalizations_domain_unique
    ON enterprise_personalizations(domain_prefix) WHERE domain_prefix <> '';

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'personalization:setting',
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'enterprise' AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 保留已保存的品牌和管理员主动授予的权限，避免回滚误删用户数据。
SELECT 1;
-- +goose StatementEnd
