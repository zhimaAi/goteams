-- +goose Up
-- 成员邀请链接表：每个租户同一时刻最多一条 active 邀请
CREATE TABLE IF NOT EXISTS member_invitations (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    token VARCHAR(64) NOT NULL,
    role_id INTEGER NOT NULL,
    invite_limit INTEGER NOT NULL,
    accepted_count INTEGER NOT NULL DEFAULT 0,
    phone_required SMALLINT NOT NULL DEFAULT 0,
    expired_at BIGINT NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE member_invitations IS '企业成员邀请链接';
COMMENT ON COLUMN member_invitations.admin_id IS '归属租户管理员ID';
COMMENT ON COLUMN member_invitations.token IS '邀请链接令牌';
COMMENT ON COLUMN member_invitations.role_id IS '被邀请人加入后的企业角色ID';
COMMENT ON COLUMN member_invitations.invite_limit IS '邀请人数上限';
COMMENT ON COLUMN member_invitations.accepted_count IS '已接受人数';
COMMENT ON COLUMN member_invitations.phone_required IS '接受邀请时电话号码是否必填: 1=必填, 0=选填';
COMMENT ON COLUMN member_invitations.expired_at IS '过期时间（Unix秒）';
COMMENT ON COLUMN member_invitations.status IS '状态: 1=进行中, 0=已终止';
COMMENT ON COLUMN member_invitations.created_by IS '发起邀请的用户ID';

CREATE UNIQUE INDEX IF NOT EXISTS idx_member_invitations_token ON member_invitations (token);
CREATE INDEX IF NOT EXISTS idx_member_invitations_admin_status ON member_invitations (admin_id, status);

-- 用户表增加邀请来源：>0 表示通过对应邀请链接加入
ALTER TABLE users ADD COLUMN IF NOT EXISTS invitation_id INTEGER NOT NULL DEFAULT 0;
COMMENT ON COLUMN users.invitation_id IS '邀请链接ID，0=非链接加入';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS invitation_id;
DROP INDEX IF EXISTS idx_member_invitations_admin_status;
DROP INDEX IF EXISTS idx_member_invitations_token;
DROP TABLE IF EXISTS member_invitations;
