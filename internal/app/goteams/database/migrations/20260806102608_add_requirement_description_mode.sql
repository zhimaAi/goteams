-- +goose Up
-- +goose StatementBegin

ALTER TABLE requirements
    ADD COLUMN description_mode VARCHAR(20) NOT NULL DEFAULT 'rich-text';

ALTER TABLE requirements
    ADD CONSTRAINT requirements_description_mode_check
    CHECK (description_mode IN ('rich-text', 'markdown'));

COMMENT ON COLUMN requirements.description_mode IS '描述编辑模式：rich-text=富文本，markdown=Markdown';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE requirements DROP COLUMN description_mode;

-- +goose StatementEnd
