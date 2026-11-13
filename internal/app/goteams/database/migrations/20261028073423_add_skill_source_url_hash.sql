-- +goose Up
-- +goose StatementBegin
ALTER TABLE skills
    ADD COLUMN source_url_hash VARCHAR(64) NOT NULL,
    ADD CONSTRAINT ck_skills_source_url_hash
        CHECK (source_url_hash = encode(sha256(convert_to(source_url, 'UTF8')), 'hex')),
    ADD CONSTRAINT uq_skills_source_url_hash UNIQUE (source_url_hash);

COMMENT ON COLUMN skills.source_url_hash IS '原始来源地址的 SHA-256 十六进制摘要，用于唯一性约束和快速查重';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE skills DROP COLUMN source_url_hash;
-- +goose StatementEnd
