-- +goose Up
-- +goose StatementBegin
WITH ranked_custom_fields AS (
    SELECT
        cf.id,
        CAST(COALESCE((
            SELECT MAX(fc.sort_order)
            FROM workspace_field_configs fc
            WHERE fc.admin_id = cf.admin_id
              AND fc.workspace_id = cf.workspace_id
              AND fc.type_key = cf.type_key
        ), -1) + ROW_NUMBER() OVER (
            PARTITION BY cf.admin_id, cf.workspace_id, cf.type_key
            ORDER BY cf.sort_order ASC, cf.id ASC
        ) AS SMALLINT) AS unified_sort_order
    FROM workspace_custom_fields cf
)
UPDATE workspace_custom_fields cf
SET sort_order = ranked.unified_sort_order
FROM ranked_custom_fields ranked
WHERE cf.id = ranked.id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
WITH ranked_custom_fields AS (
    SELECT
        id,
        CAST(ROW_NUMBER() OVER (
            PARTITION BY admin_id, workspace_id, type_key
            ORDER BY sort_order ASC, id ASC
        ) - 1 AS SMALLINT) AS custom_sort_order
    FROM workspace_custom_fields
)
UPDATE workspace_custom_fields cf
SET sort_order = ranked.custom_sort_order
FROM ranked_custom_fields ranked
WHERE cf.id = ranked.id;
-- +goose StatementEnd
