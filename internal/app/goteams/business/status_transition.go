// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"goteams/internal/app/goteams/define"
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
	Enabled int
	Fields  []StatusTransitionFieldModel
}

type statusTransitionFieldJSON struct {
	Key      string `json:"key"`
	Required any    `json:"required"`
}

type statusTransitionFormJSON struct {
	Enabled any                         `json:"enabled"`
	Fields  []statusTransitionFieldJSON `json:"fields"`
}

// defaultStatusTransitionForm returns the system preset: on, with assignee and comment optional.
func defaultStatusTransitionForm() StatusTransitionFormModel {
	return StatusTransitionFormModel{
		Enabled: 1,
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
	return form
}

func encodeStatusTransitionForm(form StatusTransitionFormModel) (string, error) {
	normalized, err := normalizeStatusTransitionForm(form)
	if err != nil {
		return ``, err
	}
	payload := statusTransitionFormJSON{
		Enabled: normalized.Enabled,
		Fields:  make([]statusTransitionFieldJSON, 0, len(normalized.Fields)),
	}
	for _, item := range normalized.Fields {
		payload.Fields = append(payload.Fields, statusTransitionFieldJSON{Key: item.Key, Required: item.Required})
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
	if len(form.Fields) > statusTransitionMaxFields {
		return StatusTransitionFormModel{}, ErrStatusTransitionFormInvalid
	}
	normalized := StatusTransitionFormModel{Enabled: enabled, Fields: make([]StatusTransitionFieldModel, 0, len(form.Fields))}
	seen := map[string]struct{}{}
	for _, item := range form.Fields {
		key := strings.TrimSpace(item.Key)
		if !isStatusTransitionFieldKey(key) {
			return StatusTransitionFormModel{}, ErrStatusTransitionFormInvalid
		}
		if _, exists := seen[key]; exists {
			return StatusTransitionFormModel{}, ErrStatusTransitionFormInvalid
		}
		seen[key] = struct{}{}
		required := 0
		if item.Required != 0 {
			required = 1
		}
		normalized.Fields = append(normalized.Fields, StatusTransitionFieldModel{Key: key, Required: required})
	}
	return normalized, nil
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
	id := cast.ToInt(strings.TrimPrefix(key, statusTransitionCustomKeyPrefix))
	if id <= 0 {
		return 0, false
	}
	return id, true
}

func statusTransitionFormByType(model WorkspaceSettingsModel, typeKey string) StatusTransitionFormModel {
	if typeKey == define.TypeKeyDefect {
		return model.DefectStatusTransition
	}
	return model.RequirementStatusTransition
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
	AdminId          int
	UserId           int
	WorkspaceId      string
	WorkItemId       int
	StatusId         int
	Comment          string
	ImageFileIds     []int
	AssigneeIds      *[]int
	VersionId        *int
	Priority         *int
	PlannedStartDate *int64
	PlannedEndDate   *int64
	CustomFields     CustomFieldValues
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
	if !HasPermission(model.AdminId, model.UserId, model.WorkspaceId, StatusTransitionEditPermission(item.TypeKey)) {
		return ErrStatusTransitionForbidden
	}
	settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: model.AdminId, WsId: model.WorkspaceId})
	if err != nil {
		return err
	}
	form := statusTransitionFormByType(settings, item.TypeKey)
	form, err = filterUnavailableStatusTransitionFields(model.AdminId, model.WorkspaceId, item.TypeKey, form)
	if err != nil {
		return err
	}
	configured := map[string]StatusTransitionFieldModel{}
	for _, field := range form.Fields {
		configured[field.Key] = field
	}
	if form.Enabled == 1 {
		for _, field := range form.Fields {
			if field.Required != 1 {
				continue
			}
			if statusTransitionValueEmpty(field.Key, model) {
				return ErrStatusTransitionFieldRequired
			}
		}
	}

	comment := strings.TrimSpace(model.Comment)
	commentID := 0
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin status transition transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	lockedData, err := getRequirementById(tx, model.AdminId, model.WorkspaceId, cast.ToString(model.WorkItemId))
	if err != nil {
		return err
	}
	if len(lockedData) == 0 {
		return ErrRequirementNotFound
	}
	beforeSnapshot, err := captureAutomationSnapshot(tx, model.AdminId, cast.ToInt(model.WorkspaceId), model.WorkItemId)
	if err != nil {
		return err
	}
	if beforeSnapshot == nil {
		return ErrRequirementNotFound
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
	return updateRequirement(tx, model.AdminId, model.WorkspaceId, cast.ToString(model.WorkItemId), updates, customFields, model.UserId, nil, define.SourceAutomation, ``)
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
