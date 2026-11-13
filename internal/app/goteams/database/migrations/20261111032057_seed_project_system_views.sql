-- +goose Up
-- 保留内置视图 ID、个人设置和已定制的系统视图，只更新仍使用原始默认值的记录。
UPDATE workspace_views v
SET name = CASE
        WHEN v.builtin_key = 'unfinished' AND v.name = '${view_builtin_unfinished}' THEN '${work_item_view_unfinished}'
        WHEN v.builtin_key = 'finished' AND v.name = '${view_builtin_finished}' THEN '${work_item_view_finished}'
        ELSE v.name
    END,
    filters = CASE
        WHEN v.builtin_key = 'finished' AND v.filters = '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_end__"]}]}' THEN
            '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_completed__"]}]}'
        ELSE v.filters
    END,
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
WHERE w.admin_id = v.admin_id AND w.id = v.workspace_id
  AND v.type_key = 'requirement' AND v.view_type = 'system'
  AND (
      (v.builtin_key = 'unfinished' AND v.name = '${view_builtin_unfinished}')
      OR (v.builtin_key = 'finished' AND (v.name = '${view_builtin_finished}' OR
          v.filters = '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_end__"]}]}'))
  );

INSERT INTO workspace_views (admin_id, workspace_id, type_key, name, view_type, builtin_key, filters, created_by, created_at, updated_at)
SELECT w.admin_id, w.id, 'requirement', v.name, 'system', v.builtin_key, v.filters, 0,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES
    ('my_processing', '${work_item_view_my_processing}', '{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]},{"field":"status","operator":"in","values":["__all_not_end__"]}]}'),
    ('my_unresolved_48h', '${work_item_view_my_unresolved_48h}', '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"my_assignment_age","operator":"within_hours","values":["48"]}]}'),
    ('my_resolved', '${work_item_view_my_resolved}', '{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]},{"field":"status","operator":"in","values":["__all_completed__"]}]}'),
    ('my_assigned', '${view_builtin_my_assigned}', '{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]}]}'),
    ('my_created', '${view_builtin_my_created}', '{"logic":"and","conditions":[{"field":"creator","operator":"in","values":["__me__"]}]}'),
    ('unresolved_48h', '${work_item_view_unresolved_48h}', '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"assignment_age","operator":"within_hours","values":["48"]}]}'),
    ('closed', '${work_item_view_closed}', '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_closed__"]}]}'),
    ('unfinished', '${work_item_view_unfinished}', '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]}]}'),
    ('finished', '${work_item_view_finished}', '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_completed__"]}]}'),
    ('all', '${view_builtin_all}', '')
) AS v(builtin_key, name, filters)
WHERE TRUE
ON CONFLICT (admin_id, workspace_id, type_key, builtin_key) WHERE builtin_key <> '' DO NOTHING;

-- +goose Down
-- 补种的视图可能已被修改或设为个人默认项，回退时保留视图及其设置。
SELECT 1;
