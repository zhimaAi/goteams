-- +goose Up
-- 团队工作查看范围从"仅成员"扩展为"组织/角色/成员"三类条目：
-- user_id 改名 target_id 以承载三类目标的 ID，target_type 标识条目类型。
ALTER TABLE role_team_work_members RENAME COLUMN user_id TO target_id;

-- 条目类型: user=成员, dept=部门, role=企业角色；存量成员数据自动迁移为 user 类型
ALTER TABLE role_team_work_members
    ADD COLUMN IF NOT EXISTS target_type VARCHAR(10) NOT NULL DEFAULT 'user';

COMMENT ON COLUMN role_team_work_members.target_id IS '目标ID（成员ID/部门ID/企业角色ID，按 target_type 区分）';
COMMENT ON COLUMN role_team_work_members.target_type IS '条目类型: user=成员, dept=部门, role=企业角色';

-- 原唯一键无法区分同 ID 的不同类型条目（如 dept:5 与 user:5），加入 target_type 重建
ALTER TABLE role_team_work_members
    DROP CONSTRAINT IF EXISTS role_team_work_members_admin_id_role_id_user_id_key;
ALTER TABLE role_team_work_members
    ADD CONSTRAINT uk_role_team_work_targets UNIQUE (admin_id, role_id, target_type, target_id);

-- +goose Down
-- 回滚时丢弃部门/角色条目，成员条目还原为旧结构
DELETE FROM role_team_work_members WHERE target_type <> 'user';

ALTER TABLE role_team_work_members
    DROP CONSTRAINT IF EXISTS uk_role_team_work_targets;
ALTER TABLE role_team_work_members DROP COLUMN IF EXISTS target_type;
ALTER TABLE role_team_work_members RENAME COLUMN target_id TO user_id;
ALTER TABLE role_team_work_members
    ADD CONSTRAINT role_team_work_members_admin_id_role_id_user_id_key UNIQUE (admin_id, role_id, user_id);
