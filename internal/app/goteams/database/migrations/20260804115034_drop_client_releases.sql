-- +goose Up
-- +goose StatementBegin
-- 客户端下载地址已改为由配置文件 [client] 段各平台独立 URL 下发（ListClientReleases），
-- client_releases 表自建表起无任何写入方，配套的发布/下载接口也已删除，故移除该表。
DROP TABLE IF EXISTS client_releases;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS client_releases (
    id SERIAL PRIMARY KEY,
    version VARCHAR(50) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    arch VARCHAR(20) NOT NULL,
    download_url TEXT NOT NULL DEFAULT '',
    sha256 VARCHAR(128) NOT NULL DEFAULT '',
    ed25519_signature TEXT NOT NULL DEFAULT '',
    manifest_json TEXT NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    created_by INTEGER NOT NULL,
    published_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(version, platform, arch)
);

COMMENT ON TABLE client_releases IS '客户端发布版本';
-- +goose StatementEnd
