package define

const (
	// TableWorkspaceStatusActions 保存工单操作名称。
	TableWorkspaceStatusActions = "workspace_status_actions"
	// TableWorkspaceStatusActionMappings 保存工单状态与操作的关系。
	TableWorkspaceStatusActionMappings = "workspace_status_action_mappings"
)

const (
	MsgStatusActionInvalid        = "workitem_status_action_invalid"
	MsgStatusActionConfigConflict = "workitem_status_action_config_conflict"
	MsgStatusActionNotFound       = "workitem_status_action_not_found"
	MsgStatusActionStale          = "workitem_status_action_stale"
)
