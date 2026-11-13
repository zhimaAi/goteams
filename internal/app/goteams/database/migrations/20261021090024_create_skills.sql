-- +goose Up
-- +goose StatementBegin
CREATE TABLE skills (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL CHECK (btrim(name) <> ''),
    remark VARCHAR(1000) NOT NULL DEFAULT '',
    icon VARCHAR(2048) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 10000),
    sharer VARCHAR(100) NOT NULL DEFAULT '',
    source_url VARCHAR(2048) NOT NULL CHECK (btrim(source_url) <> ''),
    source VARCHAR(20) NOT NULL CHECK (source IN ('github', 'skillhub', 'clawhub')),
    review_status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (review_status IN ('pending', 'approved', 'rejected')),
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

COMMENT ON TABLE skills IS '系统级技能目录，不关联租户或账号';
COMMENT ON COLUMN skills.remark IS '备注名或备注内容，最多1000个字符';
COMMENT ON COLUMN skills.icon IS '图标标识或图片地址；空值表示使用技能名称首字';
COMMENT ON COLUMN skills.sharer IS '分享者名称，普通文本，不关联账号';
COMMENT ON COLUMN skills.source IS '来源：github、skillhub、clawhub';
COMMENT ON COLUMN skills.review_status IS '审核状态：pending=待审核，approved=审核通过，rejected=审核失败';
COMMENT ON COLUMN skills.created_at IS '创建时间，Unix 秒时间戳';
COMMENT ON COLUMN skills.updated_at IS '更新时间，Unix 秒时间戳';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE skills;
-- +goose StatementEnd
