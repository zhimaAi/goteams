-- +goose Up
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS enterprise_name VARCHAR(100) NOT NULL DEFAULT '';

COMMENT ON COLUMN users.enterprise_name IS '企业名称，仅租户根管理员记录保存';

UPDATE users
SET enterprise_name = name
WHERE id = admin_id;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS enterprise_name;
