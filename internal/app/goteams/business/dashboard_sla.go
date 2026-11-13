package business

import "strconv"

// DashboardSLAStageModel 仅提供卡片选择阶段所需的信息，不暴露服务目标配置。
type DashboardSLAStageModel struct {
	Id   int
	Name string
}

// DashboardSLAOptionsModel 是当前仪表盘可配置的 SLA 阶段选项。
type DashboardSLAOptionsModel struct {
	Enabled  bool
	Timezone string
	Stages   []DashboardSLAStageModel
}

// GetDashboardSLAOptions 允许有工单查看权限的仪表盘编辑者选择阶段，不授予 SLA 设置管理权限。
func GetDashboardSLAOptions(model GetDashboardModel) (DashboardSLAOptionsModel, error) {
	result := DashboardSLAOptionsModel{Stages: []DashboardSLAStageModel{}}
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return result, err
	}
	if !dashboard.CanEdit || !HasPermission(model.AdminId, model.UserId, strconv.Itoa(model.WorkspaceId), "requirement:view") {
		return result, ErrDashboardForbidden
	}
	issueWorkspace, err := IsIssueWorkspace(model.AdminId, model.WorkspaceId)
	if err != nil {
		return result, err
	}
	if !issueWorkspace {
		return result, ErrDashboardInvalid
	}
	settings, err := getSLASettingsTx(nil, model.AdminId, model.WorkspaceId, false)
	if err != nil {
		return result, err
	}
	stages, err := listSLADefinitionsTx(nil, model.AdminId, model.WorkspaceId, true)
	if err != nil {
		return result, err
	}
	result.Enabled = settings.Enabled
	result.Timezone = settings.Timezone
	for _, stage := range stages {
		result.Stages = append(result.Stages, DashboardSLAStageModel{Id: stage.Id, Name: stage.Name})
	}
	return result, nil
}
