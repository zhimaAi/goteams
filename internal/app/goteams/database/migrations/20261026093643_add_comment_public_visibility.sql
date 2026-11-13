-- +goose Up
ALTER TABLE requirement_comments
    ADD COLUMN is_public BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN requirement_comments.is_public IS '是否对本工单邀填人公开；默认仅内部可见，与来源和回复关系独立';

-- 仅按迁移时的旧公开查询分类一次，不将内部讨论或自动化评论批量公开。
-- 父评论匹配包含完整归属，不能因不同租户、工作空间或工作项的回复关系而公开。
UPDATE requirement_comments AS c
SET is_public = TRUE
FROM workspace_issue_requirements AS i
WHERE c.admin_id = i.admin_id
    AND c.workspace_id = i.workspace_id
    AND c.requirement_id = i.requirement_id
    AND (
        c.source = 'issue_invite'
        OR c.at_invite = 1
        OR EXISTS (
            SELECT 1 FROM requirement_comments AS parent
            WHERE parent.admin_id = c.admin_id
                AND parent.workspace_id = c.workspace_id
                AND parent.requirement_id = c.requirement_id
                AND parent.id = c.parent_id
                AND parent.source = 'issue_invite'
        )
    );

-- +goose Down
-- 新公开回复的发布意图不能由旧字段恢复，拒绝回滚，避免旧代码重新公开内部内容。
-- +goose StatementBegin
DO $$
BEGIN
    RAISE EXCEPTION '公开回复可见性迁移不支持回滚，请保留新代码并通过后续迁移修复';
END;
$$;
-- +goose StatementEnd
