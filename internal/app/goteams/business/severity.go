package business

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/zhimaAi/go_tools/msql"
)

// ErrSeverityNotFound 表示当前工作空间没有指定的严重等级。
var ErrSeverityNotFound = errors.New("severity not found")

// ErrSeverityInvalid 表示严重等级不能用于当前工作项或请求。
var ErrSeverityInvalid = errors.New("invalid severity")

type severityPreset struct {
	Key, Color string
}

var severityPresets = []severityPreset{
	{"suggestion", "#808695"},
	{"normal", "#2d8cf0"},
	{"serious", "#ff8c00"},
	{"fatal", "#e54545"},
}

// SeverityModel 是严重等级选项。
type SeverityModel struct {
	Id, SortOrder        int
	Label, Color         string
	IsDefault, IsBuiltin bool
}

// SeveritySettingsModel 是工作空间严重等级设置的作用域与输入。
type SeveritySettingsModel struct {
	AdminId, WorkspaceId, Id int
	SortOrder                *int
	Lang, Name, Color        string
	Items                    []SeverityModel
}

const severityScope = " WHERE admin_id=$1 AND workspace_id=$2"

// ListSeverities 读取当前工作空间的严重等级。
func ListSeverities(model SeveritySettingsModel) (SeveritySettingsModel, error) {
	rows, err := statusActionRows(nil, "SELECT id,name,color,sort_order,is_default,preset_key FROM "+define.TableWorkspaceSeverities+
		severityScope+" ORDER BY sort_order,id", model.AdminId, model.WorkspaceId)
	if err != nil {
		return model, err
	}
	model.Items = make([]SeverityModel, 0, len(rows))
	for _, row := range rows {
		id, err := strconv.Atoi(row["id"])
		if err != nil {
			return model, fmt.Errorf("parse severity id: %w", err)
		}
		order, err := strconv.Atoi(row["sort_order"])
		if err != nil {
			return model, fmt.Errorf("parse severity sort order: %w", err)
		}
		selected, err := strconv.ParseBool(row["is_default"])
		if err != nil {
			return model, fmt.Errorf("parse severity default: %w", err)
		}
		model.Items = append(model.Items, SeverityModel{
			Id: id, SortOrder: order, Label: lib_i18n.ShowStored(model.Lang, row["name"]),
			Color: row["color"], IsDefault: selected, IsBuiltin: row["preset_key"] != "",
		})
	}
	return model, nil
}

func seedWorkspaceSeverities(tx *sql.Tx, adminId, workspaceId int, reset bool) error {
	conflict := " DO NOTHING"
	if reset {
		conflict = " DO UPDATE SET name=EXCLUDED.name,color=EXCLUDED.color,sort_order=EXCLUDED.sort_order,is_default=FALSE"
	}
	for i, preset := range severityPresets {
		_, err := statusActionExec(tx, "INSERT INTO "+define.TableWorkspaceSeverities+
			" (admin_id,workspace_id,preset_key,name,color,sort_order) VALUES ($1,$2,$3,$4,$5,$6)"+
			" ON CONFLICT (admin_id,workspace_id,preset_key)"+conflict,
			adminId, workspaceId, preset.Key, lib_i18n.WrapKey("workitem_severity_"+preset.Key), preset.Color, i+1)
		if err != nil {
			return err
		}
	}
	return nil
}

// SaveSeverity 创建或编辑严重等级。
func SaveSeverity(model SeveritySettingsModel) error {
	return withSeveritySettingsTx(model, func(tx *sql.Tx) error {
		if model.Id == 0 {
			sortOrder := 0
			if model.SortOrder == nil {
				var err error
				sortOrder, err = nextWorkflowOptionSortOrder(tx, define.TableWorkspaceSeverities, model.AdminId, model.WorkspaceId, "")
				if err != nil {
					return err
				}
			} else {
				sortOrder = *model.SortOrder
			}
			_, err := statusActionExec(tx, "INSERT INTO "+define.TableWorkspaceSeverities+
				" (admin_id,workspace_id,name,color,sort_order) VALUES ($1,$2,$3,$4,$5)",
				model.AdminId, model.WorkspaceId, model.Name, model.Color, sortOrder)
			return err
		}
		result, err := statusActionExec(tx, "UPDATE "+define.TableWorkspaceSeverities+
			" SET name=$4,color=$5,sort_order=COALESCE($6,sort_order)"+severityScope+" AND id=$3",
			model.AdminId, model.WorkspaceId, model.Id, model.Name, model.Color, model.SortOrder)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err == nil && count == 0 {
			return ErrSeverityNotFound
		}
		return err
	})
}

// DeleteSeverity 删除自定义选项，保留缺陷引用。
func DeleteSeverity(model SeveritySettingsModel) error {
	return withSeveritySettingsTx(model, func(tx *sql.Tx) error {
		rows, err := statusActionRows(tx, "SELECT preset_key FROM "+define.TableWorkspaceSeverities+
			severityScope+" AND id=$3 FOR UPDATE", model.AdminId, model.WorkspaceId, model.Id)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return ErrSeverityNotFound
		}
		if rows[0]["preset_key"] != "" {
			return ErrBuiltinWorkflowItem
		}
		_, err = statusActionExec(tx, "DELETE FROM "+define.TableWorkspaceSeverities+severityScope+" AND id=$3",
			model.AdminId, model.WorkspaceId, model.Id)
		return err
	})
}

// SetDefaultSeverity 选择默认等级，ID 为零表示无默认。
func SetDefaultSeverity(model SeveritySettingsModel) error {
	return withSeveritySettingsTx(model, func(tx *sql.Tx) error {
		if _, err := statusActionExec(tx, "UPDATE "+define.TableWorkspaceSeverities+
			" SET is_default=FALSE"+severityScope+" AND is_default", model.AdminId, model.WorkspaceId); err != nil {
			return err
		}
		if model.Id == 0 {
			return nil
		}
		result, err := statusActionExec(tx, "UPDATE "+define.TableWorkspaceSeverities+
			" SET is_default=TRUE"+severityScope+" AND id=$3", model.AdminId, model.WorkspaceId, model.Id)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err == nil && count == 0 {
			return ErrSeverityNotFound
		}
		return err
	})
}

// ResetSeverities 恢复预置配置并保留其 ID。
func ResetSeverities(model SeveritySettingsModel) error {
	return withSeveritySettingsTx(model, func(tx *sql.Tx) error {
		if _, err := statusActionExec(tx, "DELETE FROM "+define.TableWorkspaceSeverities+
			severityScope+" AND preset_key IS NULL", model.AdminId, model.WorkspaceId); err != nil {
			return err
		}
		return seedWorkspaceSeverities(tx, model.AdminId, model.WorkspaceId, true)
	})
}

func withSeveritySettingsTx(model SeveritySettingsModel, mutate func(*sql.Tx) error) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := lockStatusActionWorkspace(tx, model.AdminId, model.WorkspaceId, true); err != nil {
		return err
	}
	if err := mutate(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func resolveSeverityId(tx *sql.Tx, adminId, workspaceId int, typeKey string, selected *int) (int, error) {
	if typeKey != define.TypeKeyDefect {
		if selected != nil {
			return 0, ErrSeverityInvalid
		}
		return 0, nil
	}
	query := "SELECT id FROM " + define.TableWorkspaceSeverities + severityScope
	args := []any{adminId, workspaceId}
	if selected == nil {
		query += " AND is_default"
	} else {
		if *selected <= 0 {
			if *selected < 0 {
				return 0, ErrSeverityInvalid
			}
			return 0, nil
		}
		query += " AND id=$3 FOR SHARE"
		args = append(args, *selected)
	}
	rows, err := statusActionRows(tx, query, args...)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		if selected != nil {
			return 0, ErrSeverityNotFound
		}
		return 0, nil
	}
	return strconv.Atoi(rows[0]["id"])
}
