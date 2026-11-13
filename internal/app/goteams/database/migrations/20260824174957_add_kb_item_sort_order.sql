-- +goose Up
-- +goose StatementBegin

-- 知识库目录树手动拖拽排序：新增同级排序号。
ALTER TABLE knowledge_items ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

-- 根目录与每个文件夹分别按 id 升序一次性回填，保持首次切换手动排序时的既有展示顺序。
WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (
        PARTITION BY admin_id, workspace_id, parent_id
        ORDER BY id ASC
    ) AS rn
    FROM knowledge_items
    WHERE deleted_at = 0
)
UPDATE knowledge_items ki SET sort_order = ranked.rn
FROM ranked WHERE ki.id = ranked.id;

COMMENT ON COLUMN knowledge_items.sort_order IS '同级排序号，从 1 开始；手动拖拽排序后使用，新建节点追加到末尾';

CREATE INDEX idx_knowledge_items_siblings
    ON knowledge_items(admin_id, workspace_id, parent_id, deleted_at, sort_order, id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_knowledge_items_siblings;
ALTER TABLE knowledge_items DROP COLUMN IF EXISTS sort_order;

-- +goose StatementEnd
