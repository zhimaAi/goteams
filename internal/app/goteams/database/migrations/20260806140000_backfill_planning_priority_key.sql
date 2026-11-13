-- +goose Up
-- +goose StatementBegin

-- 回填规划优先级的稳定键。
-- 规划模块迁移（20260803100000）插入优先级时不带 key 列，落到了 DEFAULT ''；
-- 而 automation 的回填（20260721170000）版本号更早、当时还没有规划数据，
-- 导致存量工作空间的规划优先级 key 为空，与新建工作空间的种子数据不一致，
-- 自动化规则按 key 引用规划优先级时无法匹配。
UPDATE workspace_priorities
SET key = CASE name
    WHEN '${workitem_priority_urgent}' THEN 'urgent'
    WHEN '${workitem_priority_high}' THEN 'high'
    WHEN '${workitem_priority_medium}' THEN 'medium'
    WHEN '${workitem_priority_low}' THEN 'low'
    ELSE 'priority_' || id::text
END
WHERE type_key = 'planning' AND key = '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚：仅清空本次回填的规划优先级键，其它类型不受影响。
UPDATE workspace_priorities
SET key = ''
WHERE type_key = 'planning'
  AND key IN ('urgent', 'high', 'medium', 'low');

-- +goose StatementEnd
