-- +goose Up

-- 邀填工单评论标记来源；普通需求/缺陷详情评论保持为空。
ALTER TABLE requirement_comments
    ADD COLUMN IF NOT EXISTS source VARCHAR(32) NOT NULL DEFAULT '';

COMMENT ON COLUMN "requirement_comments"."source" IS '评论来源: 空=需求/缺陷详情评论, issue_invite=邀填工单评论';

CREATE INDEX IF NOT EXISTS idx_requirement_comments_scope
    ON requirement_comments(admin_id, workspace_id, requirement_id, source, id);

-- +goose Down

DROP INDEX IF EXISTS idx_requirement_comments_scope;

ALTER TABLE requirement_comments
    DROP COLUMN IF EXISTS source;
