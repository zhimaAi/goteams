package business

import (
	"errors"
	"strconv"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
)

func validateIssueFilterFields(adminId int, wsId, typeKey string, node ViewFilterNode) error {
	if typeKey != define.TypeKeyRequirement {
		return errors.New("issue filters require requirement type")
	}
	if err := validate.ValidateIssueFilter(node); err != nil {
		return err
	}
	workspaceId, err := strconv.Atoi(wsId)
	if err != nil || workspaceId <= 0 {
		return ErrSLAInvalid
	}
	issue, err := IsIssueWorkspace(adminId, workspaceId)
	if err != nil {
		return err
	}
	if !issue {
		return ErrSLAWorkspace
	}
	if node.Field != define.ViewFilterFieldSLAStatus || len(node.Values) == 0 {
		return nil
	}
	settings, err := getSLASettingsTx(nil, adminId, workspaceId, false)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return ErrSLAInvalid
	}
	stageId, _, err := validate.ParseSLAFilterValue(node.Values[0])
	if err != nil || stageId == 0 {
		return err
	}
	stages, err := listSLADefinitionsTx(nil, adminId, workspaceId, true)
	if err != nil {
		return err
	}
	for _, stage := range stages {
		if stage.Id == stageId {
			return nil
		}
	}
	return ErrSLANotFound
}

func (b *viewFilterBuilder) buildIssueFilter(node ViewFilterNode) (string, error) {
	if err := validateIssueFilterFields(b.adminId, b.wsId, b.typeKey, node); err != nil {
		return "", err
	}
	if len(node.Values) == 0 {
		return "", nil
	}
	return b.issueFilterSQL(node)
}

func (b *viewFilterBuilder) issueFilterSQL(node ViewFilterNode) (string, error) {
	if err := validate.ValidateIssueFilter(node); err != nil {
		return "", err
	}
	if len(node.Values) == 0 {
		return "", nil
	}
	if node.Field == define.ViewFilterFieldUrgeStatus {
		condition := `EXISTS (SELECT 1 FROM ` + define.TableWorkspaceIssueRequirements + ` urged
			WHERE urged.admin_id=r.admin_id AND urged.workspace_id=r.workspace_id
			AND urged.requirement_id=r.id AND urged.last_urged_at>0)`
		if node.Values[0] == "not_urged" {
			return `NOT ` + condition, nil
		}
		return condition, nil
	}
	base := `SELECT 1 FROM ` + define.TableSLACycles + ` cycle
		WHERE cycle.admin_id=r.admin_id AND cycle.workspace_id=r.workspace_id AND cycle.requirement_id=r.id`
	if node.Values[0] == "entered" {
		return `EXISTS (` + base + `)`, nil
	}
	// 值已经通过信任边界校验，阶段参数仍由驱动绑定。
	stageId, result, err := validate.ParseSLAFilterValue(node.Values[0])
	if err != nil {
		return "", err
	}
	stageParam := b.addArg(stageId)
	comparison := `>`
	if result == "completed_on_time" {
		comparison = `<=`
	}
	// 先取最近周期再判定完成，防止旧的完成结果覆盖正在运行或中断的新周期。
	return `EXISTS (` + base + ` AND cycle.stage_id=` + stageParam + `
		AND cycle.id=(SELECT latest.id FROM ` + define.TableSLACycles + ` latest
			WHERE latest.admin_id=r.admin_id AND latest.workspace_id=r.workspace_id
			AND latest.requirement_id=r.id AND latest.stage_id=` + stageParam + `
			ORDER BY (latest.state='running') DESC,latest.id DESC LIMIT 1)
		AND cycle.state='completed' AND cycle.calculation_state='ready'
		AND cycle.elapsed_seconds ` + comparison + ` cycle.limit_seconds)`, nil
}
