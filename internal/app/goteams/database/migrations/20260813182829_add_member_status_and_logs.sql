-- +goose Up
-- 已移除成员归档表：成员被移除后 users 行被物理删除，
-- 此处保留展示所需的最小信息（姓名/头像），使历史需求的创建人、处理人仍能正常显示
CREATE TABLE IF NOT EXISTS removed_users (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    name VARCHAR(100) NOT NULL DEFAULT '',
    avatar VARCHAR(500) NOT NULL DEFAULT '',
    removed_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE removed_users IS '已移除成员归档表（仅保留展示用的姓名与头像）';
COMMENT ON COLUMN removed_users.admin_id IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN removed_users.user_id IS '被移除的用户ID（users 表已删除该行）';
COMMENT ON COLUMN removed_users.name IS '被移除时的用户昵称快照';
COMMENT ON COLUMN removed_users.avatar IS '被移除时的头像URL快照';
COMMENT ON COLUMN removed_users.removed_at IS '移除时间（Unix时间戳）';

CREATE UNIQUE INDEX IF NOT EXISTS idx_removed_users_admin_user ON removed_users (admin_id, user_id);

-- 企业成员操作日志表：记录成员的新增/移除/启用/禁用
CREATE TABLE IF NOT EXISTS member_logs (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    operator_id INTEGER NOT NULL DEFAULT 0,
    target_user_id INTEGER NOT NULL,
    action VARCHAR(50) NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE member_logs IS '企业成员操作日志';
COMMENT ON COLUMN member_logs.admin_id IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN member_logs.operator_id IS '操作人用户ID（邀请链接自助加入时为该成员自身）';
COMMENT ON COLUMN member_logs.target_user_id IS '被操作的成员用户ID';
COMMENT ON COLUMN member_logs.action IS '操作类型: create=新增成员, remove=移除成员, enable=启用成员, disable=禁用成员';
COMMENT ON COLUMN member_logs.created_at IS '操作时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_member_logs_admin_created ON member_logs (admin_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_member_logs_target ON member_logs (admin_id, target_user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_member_logs_target;
DROP INDEX IF EXISTS idx_member_logs_admin_created;
DROP TABLE IF EXISTS member_logs;
DROP INDEX IF EXISTS idx_removed_users_admin_user;
DROP TABLE IF EXISTS removed_users;
