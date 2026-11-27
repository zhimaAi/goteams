-- +goose Up
ALTER TABLE workspace_field_configs
 ADD COLUMN IF NOT EXISTS edit_scope TEXT NOT NULL DEFAULT 'all' CHECK (edit_scope IN ('all','roles')),
 ADD COLUMN IF NOT EXISTS edit_role_ids INTEGER[] NOT NULL DEFAULT '{}';
ALTER TABLE workspace_custom_fields
 ADD COLUMN IF NOT EXISTS edit_scope TEXT NOT NULL DEFAULT 'all' CHECK (edit_scope IN ('all','roles')),
 ADD COLUMN IF NOT EXISTS edit_role_ids INTEGER[] NOT NULL DEFAULT '{}';

-- 描述插入标题之后，保持其他内置与自定义字段的相对顺序；已存在时不重复调整。
WITH targets AS (
 SELECT f.admin_id,f.workspace_id,f.type_key,f.sort_order FROM workspace_field_configs f
 WHERE f.field_key='title' AND f.type_key IN ('requirement','defect')
 AND NOT EXISTS (SELECT 1 FROM workspace_field_configs d WHERE d.admin_id=f.admin_id AND d.workspace_id=f.workspace_id AND d.type_key=f.type_key AND d.field_key='description')
), shifted_custom AS (
 UPDATE workspace_custom_fields c SET sort_order=c.sort_order+1 FROM targets t
 WHERE c.admin_id=t.admin_id AND c.workspace_id=t.workspace_id AND c.type_key=t.type_key AND c.sort_order>t.sort_order
), shifted_builtin AS (
 UPDATE workspace_field_configs c SET sort_order=c.sort_order+1 FROM targets t
 WHERE c.admin_id=t.admin_id AND c.workspace_id=t.workspace_id AND c.type_key=t.type_key AND c.sort_order>t.sort_order
)
INSERT INTO workspace_field_configs (admin_id,workspace_id,type_key,field_key,is_enabled,is_editable,is_locked,sort_order,created_at,updated_at)
 SELECT admin_id,workspace_id,type_key,'description',1,1,1,sort_order+1,EXTRACT(EPOCH FROM NOW())::bigint,EXTRACT(EPOCH FROM NOW())::bigint FROM targets
 ON CONFLICT (admin_id,workspace_id,type_key,field_key) DO NOTHING;

-- +goose Down
-- 保留已保存的角色策略和描述配置，避免回滚时删除用户授权数据。
SELECT 1;
