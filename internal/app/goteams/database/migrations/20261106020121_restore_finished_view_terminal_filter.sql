-- +goose Up
-- 恢复内置完成视图的终态口径，保留视图中的其他筛选和用户自建视图。
UPDATE workspace_views
SET filters = replace(filters, '"__all_completed__"', '"__all_end__"')
WHERE builtin_key = 'finished'
  AND type_key IN ('requirement', 'defect', 'iteration', 'gantt')
  AND strpos(filters, '"__all_completed__"') > 0;

-- +goose Down
-- 无法区分原有终态条件与本次恢复的条件，回退时保留视图。
SELECT 1;
