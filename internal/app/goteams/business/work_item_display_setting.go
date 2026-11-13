package business

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const (
	// WorkItemEditDisplayCreate follows the enabled attribute fields.
	WorkItemEditDisplayCreate = "create"
	// WorkItemEditDisplayView follows the resolved view-page fields.
	WorkItemEditDisplayView = "view"
	// WorkItemViewDisplayCreate follows the enabled attribute fields.
	WorkItemViewDisplayCreate = "create"
	// WorkItemViewDisplayStatus selects fields by the current status.
	WorkItemViewDisplayStatus = "status"
)

var (
	// ErrWorkItemDisplaySettingInvalid indicates an invalid display setting payload.
	ErrWorkItemDisplaySettingInvalid = errors.New("invalid work item display setting")
	detailBuiltinFieldKeys           = map[string]bool{
		"status": true, "priority": true, "version": true, "category": true, "assignee": true,
		"planned_start_date": true, "planned_end_date": true,
		"created_at": true, "created_by": true,
		"estimated_work_hours": true, "work_progress": true,
		"completed_work_hours": true, "remaining_work_hours": true,
		"overtime_work_hours": true,
	}
)

// WorkItemStatusFieldsModel stores one status's ordered field keys.
type WorkItemStatusFieldsModel struct {
	StatusId  int
	FieldKeys []string
}

// WorkItemDisplaySettingModel describes edit/view field display rules.
type WorkItemDisplaySettingModel struct {
	AdminId         int
	WorkspaceId     string
	TypeKey         string
	TemplateEnabled bool
	EditDisplayMode string
	ViewDisplayMode string
	StatusFields    []WorkItemStatusFieldsModel
	DefaultFields   []string
}

// GetWorkItemDisplaySetting returns one workspace/type display setting with missing statuses defaulted.
func GetWorkItemDisplaySetting(model WorkItemDisplaySettingModel) (WorkItemDisplaySettingModel, error) {
	defaultFields, err := enabledDetailFieldKeys(model.AdminId, model.WorkspaceId, model.TypeKey, nil)
	if err != nil {
		return WorkItemDisplaySettingModel{}, err
	}
	model.DefaultFields = defaultFields
	model.EditDisplayMode = WorkItemEditDisplayCreate
	model.ViewDisplayMode = WorkItemViewDisplayCreate
	model.TemplateEnabled, err = loadTemplateEnabled(model.AdminId, model.WorkspaceId, model.TypeKey)
	if err != nil {
		return WorkItemDisplaySettingModel{}, err
	}

	m := msql.Model(define.TableWorkItemDisplaySettings, define.Postgres)
	row, err := m.Where2([][]string{
		{"admin_id", cast.ToString(model.AdminId)},
		{"workspace_id", model.WorkspaceId},
		{"type_key", model.TypeKey},
	}).Find()
	if err != nil {
		logs.Error("get work item display setting failed, sql:%s, err:%s", m.GetLastSql(), err.Error())
		return WorkItemDisplaySettingModel{}, err
	}
	stored := map[string][]string{}
	if len(row) > 0 {
		model.EditDisplayMode = row["edit_display_mode"]
		model.ViewDisplayMode = row["view_display_mode"]
		if err = decodeStatusFieldMap(row["status_fields_json"], &stored); err != nil {
			return WorkItemDisplaySettingModel{}, err
		}
	}
	statuses, err := displaySettingStatusIDs(model.AdminId, model.WorkspaceId, model.TypeKey, nil)
	if err != nil {
		return WorkItemDisplaySettingModel{}, err
	}
	model.StatusFields = make([]WorkItemStatusFieldsModel, 0, len(statuses))
	for _, statusId := range statuses {
		keys, exists := stored[strconv.Itoa(statusId)]
		if !exists {
			keys = cloneDisplayFieldKeys(defaultFields)
		}
		model.StatusFields = append(model.StatusFields, WorkItemStatusFieldsModel{StatusId: statusId, FieldKeys: cloneDisplayFieldKeys(keys)})
	}
	return model, nil
}

// SaveWorkItemDisplaySetting validates and persists a complete workspace/type setting.
func SaveWorkItemDisplaySetting(model WorkItemDisplaySettingModel) error {
	if model.EditDisplayMode != WorkItemEditDisplayCreate && model.EditDisplayMode != WorkItemEditDisplayView {
		return ErrWorkItemDisplaySettingInvalid
	}
	if model.ViewDisplayMode != WorkItemViewDisplayCreate && model.ViewDisplayMode != WorkItemViewDisplayStatus {
		return ErrWorkItemDisplaySettingInvalid
	}
	validFields, err := validDetailFieldKeys(model.AdminId, model.WorkspaceId, model.TypeKey, nil)
	if err != nil {
		return err
	}
	statusIds, err := displaySettingStatusIDs(model.AdminId, model.WorkspaceId, model.TypeKey, nil)
	if err != nil {
		return err
	}
	validStatuses := make(map[int]bool, len(statusIds))
	for _, id := range statusIds {
		validStatuses[id] = true
	}
	stored := make(map[string][]string, len(model.StatusFields))
	for _, item := range model.StatusFields {
		if !validStatuses[item.StatusId] {
			return ErrWorkItemDisplaySettingInvalid
		}
		if _, duplicate := stored[strconv.Itoa(item.StatusId)]; duplicate {
			return ErrWorkItemDisplaySettingInvalid
		}
		seen := make(map[string]bool, len(item.FieldKeys))
		for _, key := range item.FieldKeys {
			if !validFields[key] || seen[key] {
				return ErrWorkItemDisplaySettingInvalid
			}
			seen[key] = true
		}
		stored[strconv.Itoa(item.StatusId)] = cloneDisplayFieldKeys(item.FieldKeys)
	}
	encoded, err := tool.JsonEncode(stored)
	if err != nil {
		return err
	}
	now := tool.Time2Int()
	_, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkItemDisplaySettings+`
		(admin_id, workspace_id, type_key, edit_display_mode, view_display_mode, status_fields_json, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$7)
		ON CONFLICT (admin_id, workspace_id, type_key) DO UPDATE SET
		edit_display_mode = EXCLUDED.edit_display_mode,
		view_display_mode = EXCLUDED.view_display_mode,
		status_fields_json = EXCLUDED.status_fields_json,
		updated_at = EXCLUDED.updated_at`, nil, model.AdminId, cast.ToInt(model.WorkspaceId), model.TypeKey,
		model.EditDisplayMode, model.ViewDisplayMode, encoded, now)
	if err != nil {
		logs.Error("save work item display setting failed, err:%s", err.Error())
	}
	return err
}

// InitializeDisplaySettingStatus adds a new status using the current enabled detail fields when a setting exists.
func InitializeDisplaySettingStatus(adminId int, workspaceId, typeKey string, statusId int) error {
	defaultFields, err := enabledDetailFieldKeys(adminId, workspaceId, typeKey, nil)
	if err != nil {
		return err
	}
	encoded, err := tool.JsonEncode(defaultFields)
	if err != nil {
		return err
	}
	_, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkItemDisplaySettings+`
		SET status_fields_json = jsonb_set(COALESCE(NULLIF(status_fields_json, ''), '{}')::jsonb, ARRAY[$1], $2::jsonb, true)::text,
			updated_at = $3
		WHERE admin_id = $4 AND workspace_id = $5 AND type_key = $6`, nil,
		strconv.Itoa(statusId), encoded, tool.Time2Int(), adminId, cast.ToInt(workspaceId), typeKey)
	return err
}

// RemoveDisplaySettingStatusTx removes one status entry inside the caller's transaction.
func RemoveDisplaySettingStatusTx(tx *sql.Tx, adminId int, workspaceId, typeKey string, statusId int) error {
	_, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkItemDisplaySettings+`
		SET status_fields_json = (COALESCE(NULLIF(status_fields_json, ''), '{}')::jsonb - $1)::text,
			updated_at = $2
		WHERE admin_id = $3 AND workspace_id = $4 AND type_key = $5`, tx,
		strconv.Itoa(statusId), tool.Time2Int(), adminId, cast.ToInt(workspaceId), typeKey)
	return err
}

// RemoveDisplaySettingStatus removes one status entry after status deletion commits.
func RemoveDisplaySettingStatus(adminId int, workspaceId, typeKey string, statusId int) error {
	return RemoveDisplaySettingStatusTx(nil, adminId, workspaceId, typeKey, statusId)
}

// ResetDisplaySettingStatuses replaces all status entries with current enabled detail fields.
func ResetDisplaySettingStatuses(adminId int, workspaceId, typeKey string) error {
	defaultFields, err := enabledDetailFieldKeys(adminId, workspaceId, typeKey, nil)
	if err != nil {
		return err
	}
	statusIds, err := displaySettingStatusIDs(adminId, workspaceId, typeKey, nil)
	if err != nil {
		return err
	}
	values := make(map[string][]string, len(statusIds))
	for _, statusId := range statusIds {
		values[strconv.Itoa(statusId)] = cloneDisplayFieldKeys(defaultFields)
	}
	encoded, err := tool.JsonEncode(values)
	if err != nil {
		return err
	}
	_, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkItemDisplaySettings+`
		SET status_fields_json = $1, updated_at = $2
		WHERE admin_id = $3 AND workspace_id = $4 AND type_key = $5`, nil,
		encoded, tool.Time2Int(), adminId, cast.ToInt(workspaceId), typeKey)
	return err
}

// RemoveDisplaySettingField removes a deleted custom field from every status of one workspace/type.
func RemoveDisplaySettingField(adminId int, workspaceId, typeKey, fieldKey string) error {
	setting, err := GetWorkItemDisplaySetting(WorkItemDisplaySettingModel{AdminId: adminId, WorkspaceId: workspaceId, TypeKey: typeKey})
	if err != nil {
		return err
	}
	changed := false
	for index := range setting.StatusFields {
		filtered := make([]string, 0, len(setting.StatusFields[index].FieldKeys))
		for _, key := range setting.StatusFields[index].FieldKeys {
			if key == fieldKey {
				changed = true
				continue
			}
			filtered = append(filtered, key)
		}
		setting.StatusFields[index].FieldKeys = filtered
	}
	if !changed {
		return nil
	}
	return SaveWorkItemDisplaySetting(setting)
}

func decodeStatusFieldMap(raw string, target *map[string][]string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if err := tool.JsonDecode(raw, target); err != nil {
		return fmt.Errorf("parse work item display status fields: %w", err)
	}
	return nil
}

func cloneDisplayFieldKeys(keys []string) []string {
	result := make([]string, len(keys))
	copy(result, keys)
	return result
}

func displaySettingStatusIDs(adminId int, workspaceId, typeKey string, tx *sql.Tx) ([]int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableWorkItemStatuses+`
		WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3
		ORDER BY sort_order ASC, id ASC`, tx, adminId, cast.ToInt(workspaceId), typeKey)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cast.ToInt(row["id"]))
	}
	return ids, nil
}

func validDetailFieldKeys(adminId int, workspaceId, typeKey string, tx *sql.Tx) (map[string]bool, error) {
	valid := make(map[string]bool)
	rows, err := msql.RawValues(define.Postgres, `SELECT field_key FROM `+define.TableWorkItemFieldConfigs+`
		WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3`, tx,
		adminId, cast.ToInt(workspaceId), typeKey)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if detailBuiltinFieldKeys[row["field_key"]] {
			valid[row["field_key"]] = true
		}
	}
	customRows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableCustomFields+`
		WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3`, tx,
		adminId, cast.ToInt(workspaceId), typeKey)
	if err != nil {
		return nil, err
	}
	for _, row := range customRows {
		valid["custom:"+row["id"]] = true
	}
	return valid, nil
}

func enabledDetailFieldKeys(adminId int, workspaceId, typeKey string, tx *sql.Tx) ([]string, error) {
	type orderedField struct {
		key   string
		order int
	}
	items := make([]orderedField, 0)
	rows, err := msql.RawValues(define.Postgres, `SELECT field_key, sort_order FROM `+define.TableWorkItemFieldConfigs+`
		WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND is_enabled = 1`, tx,
		adminId, cast.ToInt(workspaceId), typeKey)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if detailBuiltinFieldKeys[row["field_key"]] {
			items = append(items, orderedField{key: row["field_key"], order: cast.ToInt(row["sort_order"])})
		}
	}
	customRows, err := msql.RawValues(define.Postgres, `SELECT id, sort_order FROM `+define.TableCustomFields+`
		WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND is_enabled = 1`, tx,
		adminId, cast.ToInt(workspaceId), typeKey)
	if err != nil {
		return nil, err
	}
	for _, row := range customRows {
		items = append(items, orderedField{key: "custom:" + row["id"], order: cast.ToInt(row["sort_order"])})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].order < items[j].order })
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.key)
	}
	return result, nil
}
