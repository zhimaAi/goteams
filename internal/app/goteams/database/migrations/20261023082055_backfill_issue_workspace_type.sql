-- +goose Up
-- 只回填尚未分类且已开启邀填工单的历史项目；已有明确类型不覆盖。
-- 关联时同时匹配租户和项目，避免错误归属的应用记录触发回填。
UPDATE workspaces AS w
SET workspace_type = 'issue'
WHERE w.workspace_type = ''
  AND EXISTS (
      SELECT 1
      FROM workspace_applications AS app
      WHERE app.admin_id = w.admin_id
        AND app.workspace_id = w.id
        AND app.app_key = 'issue'
        AND app.enabled = 1
  );

COMMENT ON COLUMN workspaces.workspace_type IS '项目类型：software软件研发、agile敏捷研发、issue工单；空值为未分类的历史项目';

-- +goose Down
-- 无法区分本次回填与用户新建的工单类型，回滚不清空类型，避免误改项目分类。
