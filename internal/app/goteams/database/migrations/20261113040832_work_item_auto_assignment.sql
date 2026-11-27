-- +goose Up
CREATE TABLE work_item_auto_assignment (
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    config JSONB NOT NULL DEFAULT '{"creator_first":false,"rules":[]}',
    revision INTEGER NOT NULL DEFAULT 0,
    cursors JSONB NOT NULL DEFAULT '{}',
    PRIMARY KEY (admin_id, workspace_id)
);
COMMENT ON TABLE work_item_auto_assignment IS '工作空间自动分配配置与轮流分配游标';
COMMENT ON COLUMN work_item_auto_assignment.revision IS '管理配置修订号，不随运行游标推进而改变';
COMMENT ON COLUMN work_item_auto_assignment.cursors IS '按稳定规则ID保存上次分配的成员ID';

-- +goose Down
DROP TABLE work_item_auto_assignment;
