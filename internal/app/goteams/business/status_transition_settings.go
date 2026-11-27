package business

import (
	"database/sql"
	"strconv"

	"goteams/internal/app/goteams/define"
)

func mergeStatusTransitionUpdate(old StatusTransitionFormModel, change StatusTransitionFormUpdateModel) (StatusTransitionFormModel, error) {
	old.Enabled, old.Fields = change.Enabled, change.Fields
	if change.CustomEnabled != nil {
		old.CustomEnabled = *change.CustomEnabled
	}
	if change.StatusRules != nil {
		old.StatusRules = *change.StatusRules
	}
	return normalizeStatusTransitionForm(old)
}

func validateStatusTransitionSettingsTx(tx *sql.Tx, adminId int, wsId, typeKey string, form StatusTransitionFormModel) error {
	fields := append([]StatusTransitionFieldModel{}, form.Fields...)
	if len(form.StatusRules) > 0 {
		rows, err := statusActionRows(tx, "SELECT id FROM "+define.TableWorkItemStatuses+" WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3", adminId, wsId, typeKey)
		if err != nil {
			return err
		}
		statuses := make(map[int]bool, len(rows))
		for _, row := range rows {
			id, err := strconv.Atoi(row["id"])
			if err != nil {
				return err
			}
			statuses[id] = true
		}
		for _, rule := range form.StatusRules {
			if !statuses[rule.StatusId] {
				return ErrStatusTransitionFormInvalid
			}
			fields = append(fields, rule.Fields...)
		}
	}
	// 复用运行时字段可用性校验，拒绝跨工作区自定义字段和错误类型字段。
	filtered, err := filterUnavailableStatusTransitionFields(adminId, wsId, typeKey, StatusTransitionFormModel{Fields: fields})
	if err != nil {
		return err
	}
	if len(filtered.Fields) != len(fields) {
		return ErrStatusTransitionFormInvalid
	}
	return nil
}

func removeStatusTransitionRulesTx(tx *sql.Tx, adminId int, wsId, typeKey string, statusId int) error {
	if typeKey == define.TypeKeyPlanning {
		return nil
	}
	form, err := loadStatusTransitionForm(tx, adminId, wsId, typeKey)
	if err != nil {
		return err
	}
	rules := make([]StatusTransitionRuleModel, 0, len(form.StatusRules))
	for _, rule := range form.StatusRules {
		if statusId != 0 && rule.StatusId != statusId {
			rules = append(rules, rule)
		}
	}
	if len(rules) == len(form.StatusRules) {
		return nil
	}
	form.StatusRules = rules
	raw, err := encodeStatusTransitionForm(form)
	if err != nil {
		return err
	}
	_, err = statusActionExec(tx, "UPDATE "+define.TableWorkspaceSettings+" SET "+typeKey+"_status_transition=$3 WHERE admin_id=$1 AND workspace_id=$2", adminId, wsId, raw)
	return err
}
