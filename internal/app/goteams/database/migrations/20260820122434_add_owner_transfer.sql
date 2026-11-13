-- +goose Up
-- 默认系统管理员标记：租户内唯一，转交时在原持有人与接收人之间翻转
-- 转交后 admin_id 不变（仍作为租户隔离号），超管身份改由本字段判定
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_owner SMALLINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN "users"."is_owner" IS '默认系统管理员标记: 0=否, 1=是；租户内唯一，可转交，不再等同于 id = admin_id';

-- 存量初始化：原超管即 id = admin_id 的未删除用户
UPDATE users SET is_owner = 1 WHERE id = admin_id AND deleted_at = 0;

-- 默认项目管理员：项目内唯一，不可被移出项目，可转交
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS owner_user_id INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN "workspaces"."owner_user_id" IS '默认项目管理员用户ID: 0=未设置，读取时回落到 created_by 并写回自愈';

-- 存量初始化：优先取创建者
UPDATE workspaces SET owner_user_id = created_by WHERE created_by > 0;

-- 创建者已不在项目成员中时，回落到最早加入的成员
-- 无任何成员的项目取 0，列为 NOT NULL 不能落 NULL
UPDATE workspaces w SET owner_user_id = COALESCE((
    SELECT wm.user_id FROM workspace_members wm
    WHERE wm.workspace_id = w.id
    ORDER BY wm.joined_at ASC, wm.id ASC
    LIMIT 1
), 0)
WHERE NOT EXISTS (
    SELECT 1 FROM workspace_members m
    WHERE m.workspace_id = w.id AND m.user_id = w.created_by
);

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS is_owner;
ALTER TABLE workspaces DROP COLUMN IF EXISTS owner_user_id;
