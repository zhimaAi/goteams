-- +goose Up
-- 存储上限按累计写入字节计算。删除文件不回收，因此不能再从当前台账合计回算。
-- 存量按当前台账合计做一次底数；此前列已删除的文件无法追溯，不补回。

ALTER TABLE tenant_plans ADD COLUMN IF NOT EXISTS storage_used_bytes BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN tenant_plans.storage_used_bytes IS '累计已用存储（字节）。只增不减，删除文件不回收。';

UPDATE tenant_plans AS plans
SET storage_used_bytes = usage.total
FROM (
    SELECT admin_id, COALESCE(SUM(file_size), 0) AS total
    FROM (
        SELECT admin_id, file_size FROM enterprise_files_0
        UNION ALL SELECT admin_id, file_size FROM enterprise_files_1
        UNION ALL SELECT admin_id, file_size FROM enterprise_files_2
        UNION ALL SELECT admin_id, file_size FROM enterprise_files_3
        UNION ALL SELECT admin_id, file_size FROM enterprise_files_4
        UNION ALL SELECT admin_id, file_size FROM enterprise_files_5
        UNION ALL SELECT admin_id, file_size FROM enterprise_files_6
        UNION ALL SELECT admin_id, file_size FROM enterprise_files_7
        UNION ALL SELECT admin_id, file_size FROM enterprise_files_8
        UNION ALL SELECT admin_id, file_size FROM enterprise_files_9
    ) AS files
    GROUP BY admin_id
) AS usage
WHERE plans.admin_id = usage.admin_id
  AND plans.storage_used_bytes = 0;

-- +goose Down
-- 回滚会丢掉累计用量，无法从当前台账还原已删除文件占用过的空间。
ALTER TABLE tenant_plans DROP COLUMN IF EXISTS storage_used_bytes;
