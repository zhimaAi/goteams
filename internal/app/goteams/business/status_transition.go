// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_storage"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

const (
	statusTransitionFieldAssignee         = `assignee`
	statusTransitionFieldVersion          = `version`
	statusTransitionFieldPriority         = `priority`
	statusTransitionFieldPlannedStartDate = `planned_start_date`
	statusTransitionFieldPlannedEndDate   = `planned_end_date`
	statusTransitionFieldComment          = `comment`
	statusTransitionCustomKeyPrefix       = `custom_`
	statusTransitionFieldSeverity         = `severity_id`
	statusTransitionFieldEstimatedHours   = `estimated_work_hours`
	statusTransitionFieldResponsible      = `responsible_ids`
	statusTransitionMaxFields             = 30
	defaultStatusTransitionJSON           = `{"enabled":1,"fields":[{"key":"assignee","required":0},{"key":"comment","required":0}]}`
)

var (
	// ErrStatusTransitionFieldRequired indicates a required field on the advanced status form is empty.
	ErrStatusTransitionFieldRequired = errors.New("status transition required field empty")
	// ErrStatusTransitionFormInvalid indicates the saved or submitted transition form is invalid.
	ErrStatusTransitionFormInvalid = errors.New("status transition form invalid")
	// ErrStatusTransitionForbidden indicates the actor lacks the work item's type-specific edit permission.
	ErrStatusTransitionForbidden = errors.New("status transition forbidden")
)

var statusTransitionBuiltinKeys = map[string]struct{}{
	statusTransitionFieldSeverity:         {},
	statusTransitionFieldEstimatedHours:   {},
	statusTransitionFieldResponsible:      {},
	statusTransitionFieldAssignee:         {},
	statusTransitionFieldVersion:          {},
	statusTransitionFieldPriority:         {},
	statusTransitionFieldPlannedStartDate: {},
	statusTransitionFieldPlannedEndDate:   {},
	statusTransitionFieldComment:          {},
}

// StatusTransitionFieldModel is one field on the advanced status-transition form.
type StatusTransitionFieldModel struct {
	Key      string
	Required int
}

// StatusTransitionFormModel is the per-type advanced status-transition configuration.
type StatusTransitionFormModel struct {
	Enabled       int
	Fields        []StatusTransitionFieldModel
	CustomEnabled int
	StatusRules   []StatusTransitionRuleModel
}

// StatusTransitionRuleModel 定义进入目标状态时完整覆盖默认字段的规则。
type StatusTransitionRuleModel struct {
	StatusId int
	Fields   []StatusTransitionFieldModel
}

// StatusTransitionFormUpdateModel 保留新增配置未提供与显式清空的区别。
type StatusTransitionFormUpdateModel struct {
	Enabled       int
	Fields        []StatusTransitionFieldModel
	CustomEnabled *int
	StatusRules   *[]StatusTransitionRuleModel
}

type statusTransitionFieldJSON struct {
	Key      string `json:"key"`
	Required any    `json:"required"`
}

type statusTransitionFormJSON struct {
	Enabled       any                         `json:"enabled"`
	Fields        []statusTransitionFieldJSON `json:"fields"`
	CustomEnabled int                         `json:"custom_enabled"`
	StatusRules   []statusTransitionRuleJSON  `json:"status_rules"`
}

type statusTransitionRuleJSON struct {
	StatusId int                         `json:"status_id"`
	Fields   []statusTransitionFieldJSON `json:"fields"`
}

// defaultStatusTransitionForm returns the system preset: on, with assignee and comment optional.
func defaultStatusTransitionForm() StatusTransitionFormModel {
	return StatusTransitionFormModel{
		Enabled:     1,
		StatusRules: []StatusTransitionRuleModel{},
		Fields: []StatusTransitionFieldModel{
			{Key: statusTransitionFieldAssignee, Required: 0},
			{Key: statusTransitionFieldComment, Required: 0},
		},
	}
}

func parseStatusTransitionForm(raw string) StatusTransitionFormModel {
	if strings.TrimSpace(raw) == `` {
		return defaultStatusTransitionForm()
	}
	var payload statusTransitionFormJSON
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		logs.Error("parse status transition form failed, err:%s", err.Error())
		return defaultStatusTransitionForm()
	}
	form := StatusTransitionFormModel{Enabled: 0, Fields: make([]StatusTransitionFieldModel, 0, len(payload.Fields))}
	if cast.ToInt(payload.Enabled) != 0 {
		form.Enabled = 1
	}
	seen := map[string]struct{}{}
	for _, item := range payload.Fields {
		key := strings.TrimSpace(item.Key)
		if !isStatusTransitionFieldKey(key) {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		required := 0
		if cast.ToInt(item.Required) != 0 {
			required = 1
		}
		form.Fields = append(form.Fields, StatusTransitionFieldModel{Key: key, Required: required})
		if len(form.Fields) >= statusTransitionMaxFields {
			break
		}
	}
	form.CustomEnabled = payload.CustomEnabled
	form.StatusRules = make([]StatusTransitionRuleModel, 0, len(payload.StatusRules))
	for _, rule := range payload.StatusRules {
		fields := make([]StatusTransitionFieldModel, 0, len(rule.Fields))
		for _, field := range rule.Fields {
			fields = append(fields, StatusTransitionFieldModel{Key: field.Key, Required: cast.ToInt(field.Required)})
		}
		form.StatusRules = append(form.StatusRules, StatusTransitionRuleModel{StatusId: rule.StatusId, Fields: fields})
	}
	normalized, err := normalizeStatusTransitionForm(form)
	if err != nil {
		logs.Error("parse status transition rules failed: %s", err.Error())
		form.CustomEnabled = 0
		form.StatusRules = []StatusTransitionRuleModel{}
		return form
	}
	return normalized
}

func encodeStatusTransitionForm(form StatusTransitionFormModel) (string, error) {
	normalized, err := normalizeStatusTransitionForm(form)
	if err != nil {
		return ``, err
	}
	payload := statusTransitionFormJSON{
		Enabled:       normalized.Enabled,
		CustomEnabled: normalized.CustomEnabled,
		StatusRules:   make([]statusTransitionRuleJSON, 0, len(normalized.StatusRules)),
		Fields:        make([]statusTransitionFieldJSON, 0, len(normalized.Fields)),
	}
	for _, item := range normalized.Fields {
		payload.Fields = append(payload.Fields, statusTransitionFieldJSON{Key: item.Key, Required: item.Required})
	}
	for _, rule := range normalized.StatusRules {
		fields := make([]statusTransitionFieldJSON, 0, len(rule.Fields))
		for _, field := range rule.Fields {
			fields = append(fields, statusTransitionFieldJSON{Key: field.Key, Required: field.Required})
		}
		payload.StatusRules = append(payload.StatusRules, statusTransitionRuleJSON{StatusId: rule.StatusId, Fields: fields})
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return ``, err
	}
	return string(raw), nil
}

func normalizeStatusTransitionForm(form StatusTransitionFormModel) (StatusTransitionFormModel, error) {
	enabled := 0
	if form.Enabled != 0 {
		enabled = 1
	}
	fields, err := normalizeStatusTransitionFormFields(form.Fields)
	if err != nil {
		return StatusTransitionFormModel{}, err
	}
	normalized := StatusTransitionFormModel{Enabled: enabled, Fields: fields}
	if form.CustomEnabled != 0 && form.CustomEnabled != 1 {
		return StatusTransitionFormModel{}, ErrStatusTransitionFormInvalid
	}
	normalized.CustomEnabled = form.CustomEnabled
	normalized.StatusRules = make([]StatusTransitionRuleModel, 0, len(form.StatusRules))
	statusSeen := map[int]bool{}
	for _, rule := range form.StatusRules {
		if rule.StatusId <= 0 || statusSeen[rule.StatusId] {
			return StatusTransitionFormModel{}, ErrStatusTransitionFormInvalid
		}
		statusSeen[rule.StatusId] = true
		// 默认字段与状态覆盖字段使用同一套规范化。
		fields, err := normalizeStatusTransitionFormFields(rule.Fields)
		if err != nil {
			return StatusTransitionFormModel{}, err
		}
		normalized.StatusRules = append(normalized.StatusRules, StatusTransitionRuleModel{StatusId: rule.StatusId, Fields: fields})
	}
	return normalized, nil
}

func normalizeStatusTransitionFormFields(fields []StatusTransitionFieldModel) ([]StatusTransitionFieldModel, error) {
	if len(fields) > statusTransitionMaxFields {
		return nil, ErrStatusTransitionFormInvalid
	}
	normalized := make([]StatusTransitionFieldModel, 0, len(fields))
	seen := map[string]struct{}{}
	for _, item := range fields {
		key := strings.TrimSpace(item.Key)
		if !isStatusTransitionFieldKey(key) {
			return nil, ErrStatusTransitionFormInvalid
		}
		if _, exists := seen[key]; exists {
			return nil, ErrStatusTransitionFormInvalid
		}
		seen[key] = struct{}{}
		required := 0
		if item.Required != 0 {
			required = 1
		}
		normalized = append(normalized, StatusTransitionFieldModel{Key: key, Required: required})
	}
	return normalized, nil
}

// effectiveStatusTransitionForm 必须按规则是否存在匹配，空字段不是回退条件。
func effectiveStatusTransitionForm(form StatusTransitionFormModel, statusId int) StatusTransitionFormModel {
	if form.Enabled == 1 && form.CustomEnabled == 1 {
		for _, rule := range form.StatusRules {
			if rule.StatusId == statusId {
				form.Fields = rule.Fields
				break
			}
		}
	}
	return form
}

func isDefectStatusTransitionField(key string) bool {
	return key == statusTransitionFieldSeverity || key == statusTransitionFieldEstimatedHours || key == statusTransitionFieldResponsible
}

func isStatusTransitionFieldKey(key string) bool {
	if _, ok := statusTransitionBuiltinKeys[key]; ok {
		return true
	}
	_, ok := parseStatusTransitionCustomFieldID(key)
	return ok
}

func parseStatusTransitionCustomFieldID(key string) (int, bool) {
	if !strings.HasPrefix(key, statusTransitionCustomKeyPrefix) {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(key, statusTransitionCustomKeyPrefix))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func loadStatusTransitionForm(tx *sql.Tx, adminId int, workspaceId, typeKey string) (StatusTransitionFormModel, error) {
	column := "requirement_status_transition"
	if typeKey == define.TypeKeyDefect {
		column = "defect_status_transition"
	}
	query := "SELECT " + column + " FROM " + define.TableWorkspaceSettings + " WHERE admin_id=$1 AND workspace_id=$2"
	rows, err := msql.RawValues(define.Postgres, query, tx, adminId, workspaceId)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return StatusTransitionFormModel{}, err
	}
	if len(rows) == 0 {
		return defaultStatusTransitionForm(), nil
	}
	return parseStatusTransitionForm(rows[0][column]), nil
}

// filterUnavailableStatusTransitionFields drops disabled built-in fields and unavailable custom fields from a saved form.
func filterUnavailableStatusTransitionFields(adminId int, wsId, typeKey string, form StatusTransitionFormModel) (StatusTransitionFormModel, error) {
	fieldConfigs, err := ListFieldConfigs(ListFieldConfigsModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey})
	if err != nil {
		return StatusTransitionFormModel{}, err
	}
	builtinEnabled := map[string]int{}
	for _, field := range fieldConfigs.Items {
		builtinEnabled[field.Key] = field.Enabled
	}

	hasCustomField := false
	for _, field := range form.Fields {
		if _, ok := parseStatusTransitionCustomFieldID(field.Key); ok {
			hasCustomField = true
			break
		}
	}
	activeCustomIds := map[int]struct{}{}
	if hasCustomField {
		customFields, customErr := ListCustomFields(ListCustomFieldsModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey})
		if customErr != nil {
			return StatusTransitionFormModel{}, customErr
		}
		for _, field := range customFields.Items {
			if field.IsEnabled == 1 {
				activeCustomIds[field.Id] = struct{}{}
			}
		}
	}

	filtered := make([]StatusTransitionFieldModel, 0, len(form.Fields))
	for _, field := range form.Fields {
		if typeKey != define.TypeKeyDefect && isDefectStatusTransitionField(field.Key) {
			continue
		}
		id, custom := parseStatusTransitionCustomFieldID(field.Key)
		if custom {
			if _, active := activeCustomIds[id]; !active {
				continue
			}
		} else if field.Key != statusTransitionFieldComment {
			if enabled, configured := builtinEnabled[field.Key]; configured && enabled != 1 {
				continue
			}
		}
		filtered = append(filtered, field)
	}
	form.Fields = filtered
	return form, nil
}

// StatusTransitionEditPermission returns the edit permission key for a work item type.
func StatusTransitionEditPermission(itemType string) string {
	switch itemType {
	case define.TypeKeyDefect:
		return `defect:edit`
	default:
		return `requirement:edit`
	}
}

type statusTransitionWorkItemModel struct {
	TypeKey string
}

// TransitionWorkItemStatusModel describes applying an advanced status transition.
type TransitionWorkItemStatusModel struct {
	SeverityId         *int
	EstimatedWorkHours *float64
	ResponsibleIds     *[]int
	StatusAction       *WorkItemStatusActionModel
	AdminId            int
	UserId             int
	WorkspaceId        string
	WorkItemId         int
	StatusId           int
	Comment            string
	ImageFileIds       []int
	AssigneeIds        *[]int
	VersionId          *int
	Priority           *int
	PlannedStartDate   *int64
	PlannedEndDate     *int64
	CustomFields       CustomFieldValues
}

func getStatusTransitionWorkItem(adminId int, workspaceId string, workItemId int) (statusTransitionWorkItemModel, error) {
	m := msql.Model(define.TableRequirements, define.Postgres)
	row, err := m.Field(`type`).Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, workspaceId},
		{`id`, cast.ToString(workItemId)},
	}).Find()
	if err != nil {
		logs.Error("get status transition work item sql:%s, err:%s", m.GetLastSql(), err.Error())
		return statusTransitionWorkItemModel{}, err
	}
	if len(row) == 0 {
		return statusTransitionWorkItemModel{}, ErrRequirementNotFound
	}
	item := statusTransitionWorkItemModel{TypeKey: row[`type`]}
	if item.TypeKey != define.TypeKeyRequirement && item.TypeKey != define.TypeKeyDefect {
		return statusTransitionWorkItemModel{}, ErrStatusTransitionFormInvalid
	}
	return item, nil
}

// TransitionWorkItemStatus applies a status change together with the configured extra fields and optional comment.
func TransitionWorkItemStatus(model TransitionWorkItemStatusModel) error {
	model.Comment = lib_storage.StripOSSURLSignatures(model.Comment)
	if model.StatusId <= 0 || model.WorkItemId <= 0 {
		return ErrStatusTransitionFormInvalid
	}
	item, err := getStatusTransitionWorkItem(model.AdminId, model.WorkspaceId, model.WorkItemId)
	if err != nil {
		return err
	}
	if item.TypeKey == define.TypeKeyPlanning && !HasPermission(model.AdminId, model.UserId, model.WorkspaceId, StatusTransitionEditPermission(item.TypeKey)) {
		return ErrStatusTransitionForbidden
	}
	if item.TypeKey != define.TypeKeyPlanning {
		updates := UpdateRequirementModel{StatusId: &model.StatusId, SeverityId: model.SeverityId, EstimatedWorkHours: model.EstimatedWorkHours, ResponsibleIds: model.ResponsibleIds, AssigneeIds: model.AssigneeIds, VersionId: model.VersionId, Priority: model.Priority, PlannedStartDate: model.PlannedStartDate, PlannedEndDate: model.PlannedEndDate}
		if err := CheckWorkItemFieldEdit(CheckWorkItemFieldEditModel{AdminId: model.AdminId, UserId: model.UserId, WsId: model.WorkspaceId, ItemId: model.WorkItemId, Fields: workItemUpdateFieldKeys(updates.columns(), model.CustomFields, nil)}); err != nil {
			return err
		}
	}
	if item.TypeKey != define.TypeKeyDefect && (model.SeverityId != nil || model.EstimatedWorkHours != nil || model.ResponsibleIds != nil) {
		return ErrStatusTransitionFormInvalid
	}
	comment := strings.TrimSpace(model.Comment)
	commentID := 0
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin status transition transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if model.StatusAction != nil {
		workspaceType, err := lockStatusActionWorkspace(tx, model.AdminId, cast.ToInt(model.WorkspaceId), false)
		if err != nil {
			return err
		}
		if workspaceType != define.WorkspaceTypeIssue || item.TypeKey != define.TypeKeyRequirement {
			return ErrStatusActionWorkspace
		}
		// SLA 设置锁必须在工单行锁之前取得，沿用普通工单更新的锁顺序。
		if _, err := prepareSLAMutationTx(tx, model.AdminId, cast.ToInt(model.WorkspaceId)); err != nil {
			return err
		}
	}
	form, err := loadStatusTransitionForm(tx, model.AdminId, model.WorkspaceId, item.TypeKey)
	if err != nil {
		return err
	}
	form = effectiveStatusTransitionForm(form, model.StatusId)
	form, err = filterUnavailableStatusTransitionFields(model.AdminId, model.WorkspaceId, item.TypeKey, form)
	if err != nil {
		return err
	}
	configured := map[string]StatusTransitionFieldModel{}
	for _, field := range form.Fields {
		configured[field.Key] = field
	}
	lockedData, err := getRequirementById(tx, model.AdminId, model.WorkspaceId, cast.ToString(model.WorkItemId))
	if err != nil {
		return err
	}
	if len(lockedData) == 0 {
		return ErrRequirementNotFound
	}
	if form.Enabled == 1 {
		for _, field := range form.Fields {
			if field.Required != 1 {
				continue
			}
			empty := statusTransitionValueEmpty(field.Key, model)
			if !statusTransitionFieldProvided(field.Key, model) {
				if id, custom := parseStatusTransitionCustomFieldID(field.Key); custom {
					query := "SELECT value_text FROM " + define.TableCustomFieldValues + " WHERE admin_id=$1 AND requirement_id=$2 AND field_id=$3"
					rows, err := msql.RawValues(define.Postgres, query, tx, model.AdminId, model.WorkItemId, id)
					if err != nil {
						logs.Error("sql:%s err:%s", query, err.Error())
						return err
					}
					empty = len(rows) == 0
					if !empty {
						value := strings.TrimSpace(rows[0]["value_text"])
						empty = value == "" || value == "[]" || value == "null"
					}
				} else {
					column := map[string]string{"assignee": "assignee_ids", "version": "version_id", "estimated_work_hours": "work_hours"}[field.Key]
					if column == "" {
						column = field.Key
					}
					value := strings.TrimSpace(lockedData[column])
					empty = value == "" || value == "0" || value == "[]" || value == "{}"
					if field.Key == "estimated_work_hours" {
						empty = false
					}
				}
			}
			if empty {
				return ErrStatusTransitionFieldRequired
			}
		}
	}

	if model.StatusAction != nil {
		if err := checkWorkItemStatusAction(tx, model, lockedData); err != nil {
			return err
		}
	}
	beforeSnapshot, err := captureAutomationSnapshot(tx, model.AdminId, cast.ToInt(model.WorkspaceId), model.WorkItemId)
	if err != nil {
		return err
	}
	if beforeSnapshot == nil {
		return ErrRequirementNotFound
	}
	// 历史等级可能已删除；原值未改变时保留引用，不阻断状态流转。
	if model.SeverityId != nil && *model.SeverityId == cast.ToInt(lockedData[`severity_id`]) {
		model.SeverityId = nil
	}
	if err = applyRequirementStatusTransition(tx, model, configured, form.Enabled == 1); err != nil {
		return err
	}
	if form.Enabled == 1 && (comment != `` || len(model.ImageFileIds) > 0) {
		if _, exists := configured[statusTransitionFieldComment]; exists {
			// 状态流转附带的评论是顶层评论，不参与回复与邀填人@标记。
			if commentID, err = createCommentTx(tx, CreateCommentModel{AdminId: model.AdminId, WorkspaceId: cast.ToInt(model.WorkspaceId), RequirementId: model.WorkItemId, UserId: model.UserId, Content: comment}); err != nil {
				return err
			}
			// 同状态提交不会产生流转历史，不能将新评论挂到上一条记录。
			if cast.ToInt(lockedData["status_id"]) != model.StatusId {
				if err = linkProcessingCommentTx(tx, model, commentID); err != nil {
					return err
				}
			}
			if err = createCommentNotificationsTx(tx, model.AdminId, cast.ToInt(model.WorkspaceId), model.WorkItemId, model.UserId, comment); err != nil {
				return err
			}
			if err = bindInlineCommentFilesTx(tx, model.AdminId, cast.ToInt(model.WorkspaceId), model.WorkItemId, model.UserId, commentID, model.ImageFileIds); err != nil {
				return err
			}
		} else if len(model.ImageFileIds) > 0 {
			return ErrStatusTransitionFormInvalid
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit status transition transaction failed, err:%s", err.Error())
		return err
	}
	if err = InvalidateIssueInvitePublicCaches(model.AdminId, []int{model.WorkItemId}); err != nil {
		logs.Error("invalidate issue invite cache after status transition failed, admin_id:%d, workspace_id:%s, requirement_id:%d, err:%s", model.AdminId, model.WorkspaceId, model.WorkItemId, err.Error())
	}
	enqueueWorkItemAutomation(model.AdminId, cast.ToInt(model.WorkspaceId), model.WorkItemId, model.UserId, "update", "", beforeSnapshot)
	if commentID > 0 {
		enqueueCommentAutomation(model.AdminId, cast.ToInt(model.WorkspaceId), model.WorkItemId, model.UserId, "add",
			AutomationCommentSnapshot{ID: commentID, Content: comment, UserID: model.UserId})
	}
	return nil
}

// applyRequirementStatusTransition writes configured work-item fields inside the transition transaction.
func applyRequirementStatusTransition(tx *sql.Tx, model TransitionWorkItemStatusModel, configured map[string]StatusTransitionFieldModel, enabled bool) error {
	updates := map[string]any{`status_id`: model.StatusId}
	customFields := CustomFieldValues{}
	if enabled {
		if _, ok := configured[statusTransitionFieldSeverity]; ok && model.SeverityId != nil {
			if *model.SeverityId < 0 {
				return ErrStatusTransitionFormInvalid
			}
			updates[`severity_id`] = *model.SeverityId
		}
		if _, ok := configured[statusTransitionFieldResponsible]; ok && model.ResponsibleIds != nil {
			updates[`responsible_ids`] = *model.ResponsibleIds
		}
		if _, ok := configured[statusTransitionFieldEstimatedHours]; ok && model.EstimatedWorkHours != nil {
			if err := validate.ValidateRequirementWorkHours(*model.EstimatedWorkHours); err != nil {
				return err
			}
			updates[`work_hours`] = *model.EstimatedWorkHours
		}
		if _, ok := configured[statusTransitionFieldAssignee]; ok && model.AssigneeIds != nil {
			updates[`assignee_ids`] = *model.AssigneeIds
		}
		if _, ok := configured[statusTransitionFieldVersion]; ok && model.VersionId != nil {
			updates[`version_id`] = *model.VersionId
		}
		if _, ok := configured[statusTransitionFieldPriority]; ok && model.Priority != nil {
			updates[`priority`] = *model.Priority
		}
		if _, ok := configured[statusTransitionFieldPlannedStartDate]; ok && model.PlannedStartDate != nil {
			updates[`planned_start_date`] = *model.PlannedStartDate
		}
		if _, ok := configured[statusTransitionFieldPlannedEndDate]; ok && model.PlannedEndDate != nil {
			updates[`planned_end_date`] = *model.PlannedEndDate
		}
		customFields = statusTransitionCustomValues(configured, model.CustomFields)
	}
	return updateRequirement(tx, model.AdminId, model.WorkspaceId, cast.ToString(model.WorkItemId), updates, customFields, model.UserId, nil, define.SourceUser, ``)
}

func statusTransitionCustomValues(configured map[string]StatusTransitionFieldModel, values CustomFieldValues) CustomFieldValues {
	if len(values) == 0 {
		return CustomFieldValues{}
	}
	out := CustomFieldValues{}
	for key := range configured {
		id, ok := parseStatusTransitionCustomFieldID(key)
		if !ok {
			continue
		}
		idKey := cast.ToString(id)
		if value, exists := values[idKey]; exists {
			out[idKey] = value
		}
	}
	return out
}

func statusTransitionValueEmpty(key string, model TransitionWorkItemStatusModel) bool {
	switch key {
	case statusTransitionFieldSeverity:
		return model.SeverityId == nil || *model.SeverityId == 0
	case statusTransitionFieldEstimatedHours:
		return model.EstimatedWorkHours == nil
	case statusTransitionFieldResponsible:
		return model.ResponsibleIds == nil || len(*model.ResponsibleIds) == 0
	case statusTransitionFieldAssignee:
		return model.AssigneeIds == nil || len(*model.AssigneeIds) == 0
	case statusTransitionFieldVersion:
		return model.VersionId == nil || *model.VersionId == 0
	case statusTransitionFieldPriority:
		return model.Priority == nil || *model.Priority == 0
	case statusTransitionFieldPlannedStartDate:
		return model.PlannedStartDate == nil || *model.PlannedStartDate == 0
	case statusTransitionFieldPlannedEndDate:
		return model.PlannedEndDate == nil || *model.PlannedEndDate == 0
	case statusTransitionFieldComment:
		return strings.TrimSpace(model.Comment) == `` && len(model.ImageFileIds) == 0
	default:
		id, ok := parseStatusTransitionCustomFieldID(key)
		if !ok {
			return true
		}
		if len(model.CustomFields) == 0 {
			return true
		}
		value, exists := model.CustomFields[cast.ToString(id)]
		if !exists || value == nil {
			return true
		}
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(typed) == ``
		case []any:
			return len(typed) == 0
		case []string:
			return len(typed) == 0
		default:
			text := strings.TrimSpace(cast.ToString(value))
			return text == `` || text == `[]`
		}
	}
}

func statusTransitionFieldProvided(key string, model TransitionWorkItemStatusModel) bool {
	switch key {
	case statusTransitionFieldComment:
		return true
	case statusTransitionFieldSeverity:
		return model.SeverityId != nil
	case statusTransitionFieldEstimatedHours:
		return model.EstimatedWorkHours != nil
	case statusTransitionFieldResponsible:
		return model.ResponsibleIds != nil
	case statusTransitionFieldAssignee:
		return model.AssigneeIds != nil
	case statusTransitionFieldVersion:
		return model.VersionId != nil
	case statusTransitionFieldPriority:
		return model.Priority != nil
	case statusTransitionFieldPlannedStartDate:
		return model.PlannedStartDate != nil
	case statusTransitionFieldPlannedEndDate:
		return model.PlannedEndDate != nil
	default:
		id, ok := parseStatusTransitionCustomFieldID(key)
		if !ok {
			return false
		}
		_, provided := model.CustomFields[cast.ToString(id)]
		return provided
	}
}
