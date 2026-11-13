-- +goose Up
ALTER TABLE workspaces_settings
    ADD COLUMN requirement_completed_status_ids TEXT NOT NULL DEFAULT '',
    ADD COLUMN defect_completed_status_ids TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN workspaces_settings.requirement_completed_status_ids IS '需求完成状态ID集合，关闭状态为终态集合扣除完成集合';
COMMENT ON COLUMN workspaces_settings.defect_completed_status_ids IS '缺陷完成状态ID集合，关闭状态为终态集合扣除完成集合';
COMMENT ON COLUMN workspaces_settings.requirement_final_status_ids IS '需求终态ID集合，包含完成和关闭状态';
COMMENT ON COLUMN workspaces_settings.defect_final_status_ids IS '缺陷终态ID集合，包含完成和关闭状态';

-- 状态表没有独立的内置语义标识，使用内置标记与未翻译的国际化键识别；被重命名的状态按自定义终态保留为关闭。
-- 仅拆分原来选中的终态，不把管理员取消的状态重新设为完成。
UPDATE workspaces_settings s SET
    requirement_completed_status_ids = COALESCE((SELECT string_agg(st.id::TEXT, ',' ORDER BY st.id)
        FROM workspace_statuses st WHERE st.admin_id=s.admin_id AND st.workspace_id=s.workspace_id
        AND st.type_key='requirement' AND st.is_default=1 AND st.name='${workitem_status_done}'
        AND st.id=ANY(string_to_array(NULLIF(s.requirement_final_status_ids,''), ',')::INTEGER[])), ''),
    defect_completed_status_ids = COALESCE((SELECT string_agg(st.id::TEXT, ',' ORDER BY st.id)
        FROM workspace_statuses st WHERE st.admin_id=s.admin_id AND st.workspace_id=s.workspace_id
        AND st.type_key='defect' AND st.is_default=1 AND st.name IN ('${workitem_status_resolved}', '${workitem_status_done}')
        AND st.id=ANY(string_to_array(NULLIF(s.defect_final_status_ids,''), ',')::INTEGER[])), '');

-- 内置完成视图改用完成集合；用户自建的终态筛选保持原意。
UPDATE workspace_views SET filters=replace(filters, '__all_end__', '__all_completed__')
WHERE builtin_key='finished' AND type_key IN ('requirement','defect','iteration','gantt');

-- 完成数量按每个工作项所属工作空间的完成配置重算，保留关闭项在总数中。
UPDATE versions v SET completed_count=(SELECT COUNT(*) FROM requirements r
    JOIN workspaces_settings s ON s.admin_id=r.admin_id AND s.workspace_id=r.workspace_id
    WHERE r.admin_id=v.admin_id AND r.version_id=v.id
    AND r.status_id=ANY(string_to_array(NULLIF(CASE WHEN r.type='defect'
        THEN s.defect_completed_status_ids ELSE s.requirement_completed_status_ids END,''), ',')::INTEGER[]));

-- +goose Down
-- 完成与关闭的终态并集始终保留，回退只删除分类，不会重新打开已有终态工作项。
UPDATE workspace_views SET filters=replace(filters, '__all_completed__', '__all_end__')
WHERE builtin_key='finished' AND type_key IN ('requirement','defect','iteration','gantt');
ALTER TABLE workspaces_settings DROP COLUMN requirement_completed_status_ids, DROP COLUMN defect_completed_status_ids;
COMMENT ON COLUMN workspaces_settings.requirement_final_status_ids IS '需求结束状态ID集合';
COMMENT ON COLUMN workspaces_settings.defect_final_status_ids IS '缺陷结束状态ID集合';
UPDATE versions v SET completed_count=(SELECT COUNT(*) FROM requirements r
    JOIN workspaces_settings s ON s.admin_id=r.admin_id AND s.workspace_id=r.workspace_id
    WHERE r.admin_id=v.admin_id AND r.version_id=v.id
    AND r.status_id=ANY(string_to_array(NULLIF(CASE WHEN r.type='defect'
        THEN s.defect_final_status_ids ELSE s.requirement_final_status_ids END,''), ',')::INTEGER[]));
