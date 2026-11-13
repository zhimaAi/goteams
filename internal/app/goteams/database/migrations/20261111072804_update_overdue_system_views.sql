-- +goose Up
-- 只更新仍使用旧默认筛选的两项内置视图，保留名称、视图 ID 和用户偏好。
UPDATE workspace_views AS v
SET filters = rules.new_filters,
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
FROM (VALUES
    ('unresolved_48h',
     '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"assignment_age","operator":"within_hours","values":["48"]}]}',
     '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"created_at","operator":"older_than_hours","values":["48"]}]}'),
    ('my_unresolved_48h',
     '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"my_assignment_age","operator":"within_hours","values":["48"]}]}',
     '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"my_assignment_age","operator":"older_than_hours","values":["48"]}]}')
) AS rules(builtin_key, old_filters, new_filters)
CROSS JOIN workspaces AS w
WHERE v.admin_id = w.admin_id AND v.workspace_id = w.id
  AND v.type_key = 'requirement' AND v.view_type = 'system'
  AND v.builtin_key = rules.builtin_key AND v.filters = rules.old_filters;

-- +goose Down
-- 只回退仍使用新默认筛选的两项内置视图，保留上线后编辑的其他筛选。
UPDATE workspace_views AS v
SET filters = rules.new_filters,
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
FROM (VALUES
    ('unresolved_48h',
     '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"created_at","operator":"older_than_hours","values":["48"]}]}',
     '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"assignment_age","operator":"within_hours","values":["48"]}]}'),
    ('my_unresolved_48h',
     '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"my_assignment_age","operator":"older_than_hours","values":["48"]}]}',
     '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"my_assignment_age","operator":"within_hours","values":["48"]}]}')
) AS rules(builtin_key, old_filters, new_filters)
CROSS JOIN workspaces AS w
WHERE v.admin_id = w.admin_id AND v.workspace_id = w.id
  AND v.type_key = 'requirement' AND v.view_type = 'system'
  AND v.builtin_key = rules.builtin_key AND v.filters = rules.old_filters;
