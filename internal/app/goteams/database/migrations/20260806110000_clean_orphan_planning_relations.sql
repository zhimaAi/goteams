-- +goose Up
-- +goose StatementBegin

-- 清理悬空的规划关联。
-- 删除工作项时未级联清理 planning_relations（其外键列是 item_id，
-- 不在 DeleteRequirement 的统一清理循环里），残留记录会让规划列表的
-- 「有关联」筛选把已删除的工作项也算进去。代码侧已补上级联删除，
-- 这里清理存量脏数据。
-- anti-join 删除在 PostgreSQL 中以 NOT EXISTS 表达最直接，且为一次性数据修复。
DELETE FROM planning_relations pr
WHERE NOT EXISTS (
    SELECT 1 FROM requirements r
    WHERE r.id = pr.item_id AND r.admin_id = pr.admin_id
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 数据清理不可逆：被删除的都是指向已不存在工作项的悬空记录，无还原目标。
SELECT 1;

-- +goose StatementEnd
