-- +goose Up
-- 成员邮箱：与登录账号分离，用于找回密码与联系；空串表示未绑定
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email VARCHAR(255) NOT NULL DEFAULT '';

COMMENT ON COLUMN users.email IS '企业成员邮箱，空=未绑定';

-- 与 idx_users_username 对齐：空邮箱不占用唯一性，移除成员时清空邮箱即可释放
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email) WHERE email <> '';

-- 邀请链接：接受邀请时邮箱是否必填
ALTER TABLE member_invitations
    ADD COLUMN IF NOT EXISTS email_required SMALLINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN member_invitations.email_required IS '接受邀请时邮箱是否必填: 1=必填, 0=选填';

-- +goose Down
ALTER TABLE member_invitations DROP COLUMN IF EXISTS email_required;
DROP INDEX IF EXISTS idx_users_email;
ALTER TABLE users DROP COLUMN IF EXISTS email;
