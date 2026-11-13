package define

const (
	// NotificationEventWorkItemSLA 表示新工单 SLA 的站内通知。
	NotificationEventWorkItemSLA = "work_item_sla"
	MsgSLAInvalid                = "sla_invalid"
	MsgSLAConflict               = "sla_conflict"
	MsgSLAWorkspace              = "sla_workspace"
	MsgSLANotFound               = "sla_not_found"
	// TableSLASettings 保存工单空间全部 SLA 配置和启停区间。
	TableSLASettings = "workspace_sla_settings"
	// TableSLACycles 保存独立阶段执行周期。
	TableSLACycles = "requirement_sla_cycles"
	// TableSLAActions 保存通知计划和发送结果。
	TableSLAActions = "requirement_sla_action_executions"
)
