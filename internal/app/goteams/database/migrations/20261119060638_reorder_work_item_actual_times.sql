-- +goose Up
-- 只修复旧补种将两个时间字段连续追加在末尾的空间；已移到其他位置的配置保持不变。
-- 内置与自定义字段共用顺序，移动时间字段时保留所有其他字段的相对顺序及属性。
WITH fields AS (
    SELECT id,admin_id,workspace_id,'builtin' AS kind,field_key,sort_order
    FROM workspace_field_configs WHERE type_key='requirement'
    UNION ALL
    SELECT id,admin_id,workspace_id,'custom' AS kind,'' AS field_key,sort_order
    FROM workspace_custom_fields WHERE type_key='requirement'
), ranked AS (
    SELECT *,ROW_NUMBER() OVER (PARTITION BY admin_id,workspace_id ORDER BY sort_order,kind,id) AS field_position,
        COUNT(*) OVER (PARTITION BY admin_id,workspace_id) AS field_count
    FROM fields
), eligible AS (
    SELECT admin_id,workspace_id,MAX(field_position) FILTER (WHERE field_key='estimated_work_hours') AS anchor_position
    FROM ranked
    GROUP BY admin_id,workspace_id
    HAVING MAX(field_position) FILTER (WHERE field_key='started_at')=MAX(field_count)-1
        AND MAX(field_position) FILTER (WHERE field_key='ended_at')=MAX(field_count)
        AND MAX(field_position) FILTER (WHERE field_key='estimated_work_hours') IS NOT NULL
), reordered AS (
    SELECT r.id,r.admin_id,r.workspace_id,r.kind,
        ROW_NUMBER() OVER (PARTITION BY r.admin_id,r.workspace_id ORDER BY
            CASE WHEN r.field_key IN ('started_at','ended_at') THEN e.anchor_position ELSE r.field_position END,
            CASE r.field_key WHEN 'started_at' THEN 1 WHEN 'ended_at' THEN 2 ELSE 0 END)-1 AS new_order
    FROM ranked r JOIN eligible e ON e.admin_id=r.admin_id AND e.workspace_id=r.workspace_id
), updated_builtin AS (
    UPDATE workspace_field_configs f SET sort_order=r.new_order,updated_at=EXTRACT(EPOCH FROM NOW())::BIGINT
    FROM reordered r
    WHERE r.kind='builtin' AND f.id=r.id AND f.admin_id=r.admin_id AND f.workspace_id=r.workspace_id
        AND f.type_key='requirement' AND f.sort_order IS DISTINCT FROM r.new_order
    RETURNING f.id
)
UPDATE workspace_custom_fields f SET sort_order=r.new_order,updated_at=EXTRACT(EPOCH FROM NOW())::BIGINT
FROM reordered r
WHERE r.kind='custom' AND f.id=r.id AND f.admin_id=r.admin_id AND f.workspace_id=r.workspace_id
    AND f.type_key='requirement' AND f.sort_order IS DISTINCT FROM r.new_order;

-- +goose Down
-- 无法区分迁移后的用户主动排序，不回写旧顺序以免覆盖用户设置。
SELECT 1;
