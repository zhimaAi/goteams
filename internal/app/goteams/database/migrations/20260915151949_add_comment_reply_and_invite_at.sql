-- +goose Up

-- 评论回复关系：parent_id 指向被回复的评论，0 表示顶层评论。
ALTER TABLE requirement_comments
    ADD COLUMN IF NOT EXISTS parent_id INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN "requirement_comments"."parent_id" IS '回复的目标评论ID: 0=顶层评论';

-- 是否@了本工单的邀填人。邀填人只有 openid 没有 user_id，且「邀填人」是
-- 多语言固定词，无法在 SQL 里可靠匹配正文，因此单独落库标记。
ALTER TABLE requirement_comments
    ADD COLUMN IF NOT EXISTS at_invite SMALLINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN "requirement_comments"."at_invite" IS '是否@了本工单邀填人: 0=否, 1=是';

CREATE INDEX IF NOT EXISTS idx_requirement_comments_parent
    ON requirement_comments(admin_id, workspace_id, requirement_id, parent_id);

-- +goose Down

DROP INDEX IF EXISTS idx_requirement_comments_parent;

ALTER TABLE requirement_comments
    DROP COLUMN IF EXISTS at_invite;

ALTER TABLE requirement_comments
    DROP COLUMN IF EXISTS parent_id;
