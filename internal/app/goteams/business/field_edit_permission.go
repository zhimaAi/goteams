package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

// ErrFieldEditForbidden 表示当前成员没有本次字段修改权限。
var ErrFieldEditForbidden = errors.New("field edit forbidden")

// ErrFieldEditPolicyInvalid 表示字段编辑角色配置无效。
var ErrFieldEditPolicyInvalid = errors.New("invalid field edit policy")

// FieldEditPolicyModel 是独立于成员候选范围的字段编辑策略。
type FieldEditPolicyModel struct {
	EditScope   string
	EditRoleIds []int
}

func fieldEditPolicy(scope, ids string) (FieldEditPolicyModel, error) {
	policy := FieldEditPolicyModel{EditScope: scope, EditRoleIds: []int{}}
	if err := json.Unmarshal([]byte(ids), &policy.EditRoleIds); err != nil {
		return policy, fmt.Errorf("parse field edit roles: %w", err)
	}
	if scope != "all" && scope != "roles" {
		return policy, ErrFieldEditPolicyInvalid
	}
	return policy, nil
}

// SupportsFieldEditRoles 区分可配置授权的字段与系统只读/范围外字段。
func SupportsFieldEditRoles(typeKey, key string) bool {
	if typeKey != define.TypeKeyRequirement && typeKey != define.TypeKeyDefect {
		return false
	}
	switch key {
	case "title", "description", "version", "status", "priority", "assignee", "planned_start_date", "planned_end_date", "estimated_work_hours":
		return true
	case "category":
		return typeKey == define.TypeKeyRequirement
	case "severity_id", "responsible_ids":
		return typeKey == define.TypeKeyDefect
	}
	return strings.HasPrefix(key, "custom:")
}

func (p FieldEditPolicyModel) matches(roleId int) bool {
	return roleId > 0 && (p.EditScope == "all" || (p.EditScope == "roles" && slices.Contains(p.EditRoleIds, roleId)))
}

func validateFieldEditPolicy(tx *sql.Tx, adminId int, wsId string, policy FieldEditPolicyModel) (FieldEditPolicyModel, error) {
	if policy.EditScope == "all" {
		policy.EditRoleIds = []int{}
		return policy, nil
	}
	if policy.EditScope != "roles" || len(policy.EditRoleIds) == 0 {
		return policy, ErrFieldEditPolicyInvalid
	}
	ids, err := validatedTemplateVisibleRoles(tx, adminId, cast.ToInt(wsId), policy.EditRoleIds)
	if err != nil {
		if errors.Is(err, ErrTemplateVisibleRolesInvalid) {
			return policy, ErrFieldEditPolicyInvalid
		}
		return policy, err
	}
	policy.EditRoleIds = ids
	return policy, nil
}

// FieldEditAccessModel 是工作空间内某类字段的当前用户能力。
type FieldEditAccessModel struct {
	AdminId int
	UserId  int
	WsId    string
	TypeKey string
	RoleId  int
	CanView bool
	CanEdit bool
}

// LoadFieldEditAccess 复用当前有效工作空间角色与类型权限，不信任前端角色信息。
func LoadFieldEditAccess(model FieldEditAccessModel) (FieldEditAccessModel, error) {
	roleId, err := TemplateViewerRole(model.AdminId, cast.ToInt(model.WsId), model.UserId)
	if err != nil {
		return model, err
	}
	model.RoleId = roleId
	model.CanView = HasPermission(model.AdminId, model.UserId, model.WsId, model.TypeKey+":view")
	model.CanEdit = HasPermission(model.AdminId, model.UserId, model.WsId, model.TypeKey+":edit")
	return model, nil
}

func (a FieldEditAccessModel) canEditField(key string, policy FieldEditPolicyModel, manuallyEditable bool) bool {
	if !manuallyEditable {
		return false
	}
	if SupportsFieldEditRoles(a.TypeKey, key) {
		return a.CanView && policy.matches(a.RoleId)
	}
	return a.CanEdit
}

// CheckWorkItemFieldEditModel 描述一次人工更新涉及的对象及字段。
type CheckWorkItemFieldEditModel struct {
	AdminId int
	UserId  int
	WsId    string
	ItemId  int
	Fields  []string
}

// CheckWorkItemFieldEdit 校验所有提交字段；任何字段失败都在落库前拒绝。
func CheckWorkItemFieldEdit(model CheckWorkItemFieldEditModel) error {
	return checkWorkItemFieldEditTx(nil, model)
}

func checkWorkItemFieldEditTx(tx *sql.Tx, model CheckWorkItemFieldEditModel) error {
	item, err := getRequirementById(tx, model.AdminId, model.WsId, strconv.Itoa(model.ItemId))
	if err != nil {
		return err
	}
	if len(item) == 0 {
		return ErrRequirementNotFound
	}
	access, err := LoadFieldEditAccess(FieldEditAccessModel{AdminId: model.AdminId, UserId: model.UserId, WsId: model.WsId, TypeKey: item["type"]})
	if err != nil {
		return err
	}
	if !access.CanView || (access.RoleId == 0 && !IsTenantOwner(model.AdminId, model.UserId)) {
		return ErrFieldEditForbidden
	}
	visible, err := CheckRequirementVisible(model.AdminId, model.WsId, strconv.Itoa(model.ItemId), model.UserId)
	if err != nil {
		return err
	}
	if !visible {
		return ErrFieldEditForbidden
	}
	for _, key := range model.Fields {
		if !SupportsFieldEditRoles(item["type"], key) {
			if access.CanEdit {
				continue
			}
			return ErrFieldEditForbidden
		}
		table, clause, value := define.TableWorkItemFieldConfigs, "field_key", any(key)
		if strings.HasPrefix(key, "custom:") {
			table, clause = define.TableCustomFields, "id"
			id, e := strconv.Atoi(strings.TrimPrefix(key, "custom:"))
			if e != nil || id <= 0 {
				return ErrFieldEditForbidden
			}
			value = id
		}
		manual := "is_editable"
		if strings.HasPrefix(key, "custom:") {
			manual = "1"
		}
		query := "SELECT " + manual + " AS manually_editable,edit_scope,array_to_json(edit_role_ids)::text AS edit_role_ids FROM " + table + " WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND " + clause + "=$4"
		if tx != nil {
			query += " FOR SHARE"
		}
		rows, err := msql.RawValues(define.Postgres, query, tx, model.AdminId, model.WsId, item["type"], value)
		if err != nil {
			logs.Error("sql:%s err:%s", query, err.Error())
			return err
		}
		if len(rows) != 1 {
			return ErrFieldEditForbidden
		}
		policy, err := fieldEditPolicy(rows[0]["edit_scope"], rows[0]["edit_role_ids"])
		if err != nil {
			return err
		}
		if !access.canEditField(key, policy, rows[0]["manually_editable"] == "1" || key == "title" || key == "status" || key == "description") {
			return ErrFieldEditForbidden
		}
	}
	return nil
}

func workItemUpdateFieldKeys(updates map[string]any, customFields map[string]any, imageFileIds []int) []string {
	keys := make([]string, 0, len(updates)+len(customFields)+1)
	for key := range updates {
		switch key {
		case "status_id":
			key = "status"
		case "version_id":
			key = "version"
		case "assignee_ids":
			key = "assignee"
		case "work_hours":
			key = "estimated_work_hours"
		case "category_id":
			key = "category"
		case "description_mode":
			key = "description"
		}
		keys = append(keys, key)
	}
	for id := range customFields {
		keys = append(keys, "custom:"+id)
	}
	if len(imageFileIds) > 0 {
		keys = append(keys, "description")
	}
	return keys
}

// SaveFieldEditPolicyModel 保存单个内置或自定义字段的编辑角色限制。
type SaveFieldEditPolicyModel struct {
	AdminId int
	WsId    string
	TypeKey string
	Kind    string
	Key     string
	Id      int
	Policy  FieldEditPolicyModel
}

// SaveFieldEditPolicy 不改动字段类型、候选成员和显示配置。
func SaveFieldEditPolicy(model SaveFieldEditPolicyModel) error {
	key := model.Key
	table, column, value := define.TableWorkItemFieldConfigs, "field_key", any(model.Key)
	if model.Kind == "custom" && model.Id > 0 {
		key = "custom:" + strconv.Itoa(model.Id)
		table, column, value = define.TableCustomFields, "id", model.Id
	} else if model.Kind != "builtin" {
		return ErrFieldEditPolicyInvalid
	}
	if !SupportsFieldEditRoles(model.TypeKey, key) {
		return ErrFieldEditPolicyInvalid
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	policy, err := validateFieldEditPolicy(tx, model.AdminId, model.WsId, model.Policy)
	if err != nil {
		return err
	}
	query := "UPDATE " + table + " SET edit_scope=$1,edit_role_ids=$2,updated_at=EXTRACT(EPOCH FROM NOW())::bigint WHERE admin_id=$3 AND workspace_id=$4 AND type_key=$5 AND " + column + "=$6"
	result, err := msql.RawExec(define.Postgres, query, tx, policy.EditScope, pq.Array(policy.EditRoleIds), model.AdminId, model.WsId, model.TypeKey, value)
	if err != nil {
		logs.Error("sql:%s err:%s", query, err.Error())
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrFieldEditPolicyInvalid
	}
	return tx.Commit()
}
