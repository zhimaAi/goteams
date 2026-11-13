-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- 项目标识唯一性收敛到租户内：不同企业之间允许项目重名
-- 原索引只按 slug 建，导致 A 企业建过的项目名 B 企业无法再用
-- ============================================================

DROP INDEX IF EXISTS workspaces_slug_active_key;
CREATE UNIQUE INDEX IF NOT EXISTS workspaces_slug_active_key
    ON workspaces(admin_id, slug)
    WHERE COALESCE(archived, 0) = 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS workspaces_slug_active_key;
CREATE UNIQUE INDEX IF NOT EXISTS workspaces_slug_active_key
    ON workspaces(slug)
    WHERE COALESCE(archived, 0) = 0;

-- +goose StatementEnd
