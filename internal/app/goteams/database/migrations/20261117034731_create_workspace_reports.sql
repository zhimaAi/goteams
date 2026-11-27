-- +goose Up
CREATE TABLE workspace_reports (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL CHECK (admin_id > 0),
    workspace_id INTEGER NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    report_type VARCHAR(20) NOT NULL
        CHECK (report_type IN ('work-hours', 'defects')),
    created_by INTEGER NOT NULL CHECK (created_by > 0),
    created_by_name TEXT NOT NULL,
    created_at BIGINT NOT NULL CHECK (created_at > 0),
    period_start_at BIGINT NOT NULL,
    period_end_at BIGINT NOT NULL,
    timezone VARCHAR(100) NOT NULL CHECK (timezone <> ''),
    filters JSONB NOT NULL CHECK (jsonb_typeof(filters) = 'object'),
    snapshot_version SMALLINT NOT NULL DEFAULT 1 CHECK (snapshot_version > 0),
    summary JSONB NOT NULL CHECK (jsonb_typeof(summary) = 'object'),
    request_id UUID NOT NULL,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    CHECK (period_start_at < period_end_at),
    UNIQUE (admin_id, workspace_id, id),
    UNIQUE (admin_id, workspace_id, report_type, created_by, request_id)
);

CREATE INDEX idx_workspace_reports_history
    ON workspace_reports (admin_id, workspace_id, report_type, created_at DESC, id DESC);

CREATE INDEX idx_workspace_reports_workspace
    ON workspace_reports (workspace_id);

CREATE TABLE workspace_report_items (
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    report_id BIGINT NOT NULL,
    work_item_id INTEGER NOT NULL CHECK (work_item_id > 0),
    item_snapshot JSONB NOT NULL CHECK (jsonb_typeof(item_snapshot) = 'object'),
    contributions JSONB NOT NULL CHECK (jsonb_typeof(contributions) = 'array'),
    PRIMARY KEY (admin_id, workspace_id, report_id, work_item_id),
    FOREIGN KEY (admin_id, workspace_id, report_id)
        REFERENCES workspace_reports (admin_id, workspace_id, id) ON DELETE CASCADE
);

COMMENT ON TABLE workspace_reports IS '工作区报表生成记录';
COMMENT ON COLUMN workspace_reports.id IS '报表ID';
COMMENT ON COLUMN workspace_reports.admin_id IS '租户ID';
COMMENT ON COLUMN workspace_reports.workspace_id IS '所属工作区ID';
COMMENT ON COLUMN workspace_reports.report_type IS '报表类型';
COMMENT ON COLUMN workspace_reports.created_by IS '生成者ID';
COMMENT ON COLUMN workspace_reports.created_by_name IS '生成时的生成者名称';
COMMENT ON COLUMN workspace_reports.created_at IS '生成时间（Unix秒）';
COMMENT ON COLUMN workspace_reports.period_start_at IS '统计区间起点（Unix秒，含）';
COMMENT ON COLUMN workspace_reports.period_end_at IS '统计区间终点（Unix秒，不含）';
COMMENT ON COLUMN workspace_reports.timezone IS '统计区间采用的时区';
COMMENT ON COLUMN workspace_reports.filters IS '已选条件及当时名称';
COMMENT ON COLUMN workspace_reports.snapshot_version IS '快照格式版本';
COMMENT ON COLUMN workspace_reports.summary IS '按维度分组的汇总快照';
COMMENT ON COLUMN workspace_reports.request_id IS '生成请求的幂等标识';
COMMENT ON COLUMN workspace_reports.request_hash IS '规范化请求的SHA-256指纹';

COMMENT ON TABLE workspace_report_items IS '工作区报表工作项快照';
COMMENT ON COLUMN workspace_report_items.admin_id IS '租户ID';
COMMENT ON COLUMN workspace_report_items.workspace_id IS '所属工作区ID';
COMMENT ON COLUMN workspace_report_items.report_id IS '关联报表ID';
COMMENT ON COLUMN workspace_report_items.work_item_id IS '原工作项ID';
COMMENT ON COLUMN workspace_report_items.item_snapshot IS '生成时的工作项字段快照';
COMMENT ON COLUMN workspace_report_items.contributions IS '工作项对各分组的计数和工时贡献';
-- +goose Down
LOCK TABLE workspace_reports, workspace_report_items IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM workspace_reports) THEN
        RAISE EXCEPTION '工作区报表已有历史数据，禁止回退建表迁移';
    END IF;
END
$$;
-- +goose StatementEnd
DROP TABLE workspace_report_items;
DROP TABLE workspace_reports;
