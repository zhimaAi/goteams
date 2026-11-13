-- +goose Up

-- 开源版系统授权单例表：固定仅一行（id=1）。
-- 实例级安装元数据，不属于租户业务表，禁止混入 admin_id/workspace_id 租户数据。
-- license_key 为每次验签的可信源，其余字段均为验签通过后的查询投影，不可单独修改生效。
CREATE TABLE IF NOT EXISTS commercial_licenses (
    id SMALLINT PRIMARY KEY,
    license_key TEXT NOT NULL DEFAULT '',
    license_no VARCHAR(64) NOT NULL DEFAULT '',
    activation_mode VARCHAR(16) NOT NULL DEFAULT '',
    corp_name VARCHAR(100) NOT NULL DEFAULT '',
    device_code VARCHAR(64) NOT NULL DEFAULT '',
    issued_at BIGINT NOT NULL DEFAULT 0,
    not_before BIGINT NOT NULL DEFAULT 0,
    expire_at BIGINT NOT NULL DEFAULT 0,
    revision INTEGER NOT NULL DEFAULT 0,
    claims_hash VARCHAR(64) NOT NULL DEFAULT '',
    online_check_receipt TEXT NOT NULL DEFAULT '',
    last_online_sync_at BIGINT NOT NULL DEFAULT 0,
    last_seen_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE commercial_licenses IS '开源版系统授权单例（id=1，安装实例级元数据，非租户表）';
COMMENT ON COLUMN commercial_licenses.license_key IS '完整授权 Key（GTLIC1 三段式），验签可信源';
COMMENT ON COLUMN commercial_licenses.license_no IS '授权编号（GTL-YYYYMMDD-XXXXXXXX）';
COMMENT ON COLUMN commercial_licenses.activation_mode IS '授权方式：online=在线，offline=离线';
COMMENT ON COLUMN commercial_licenses.device_code IS '规范化设备码（GTDEV1-XXXX-XXXX-XXXX-XXXX）';
COMMENT ON COLUMN commercial_licenses.issued_at IS 'Key 签发时间（Unix秒）';
COMMENT ON COLUMN commercial_licenses.not_before IS 'Key 生效时间（Unix秒）';
COMMENT ON COLUMN commercial_licenses.expire_at IS 'Key 到期时间（Unix秒），必为未来时间点，无永久授权';
COMMENT ON COLUMN commercial_licenses.revision IS '授权版本号，低版本不得回退覆盖';
COMMENT ON COLUMN commercial_licenses.claims_hash IS '已验签 claims 段的 SHA-256 十六进制，审计用';
COMMENT ON COLUMN commercial_licenses.online_check_receipt IS '运营后台签名的在线校验回执（试用强制在线模式）';
COMMENT ON COLUMN commercial_licenses.last_online_sync_at IS '最后一次成功在线校验时间（Unix秒）';
COMMENT ON COLUMN commercial_licenses.last_seen_at IS '最后一次授权门禁通过时间（Unix秒）';

-- +goose Down

DROP TABLE IF EXISTS commercial_licenses;
