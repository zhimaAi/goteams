// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const issueInviteTemplateFields = `t.id, t.type_key, t.name, t.description_mode,
	t.fields_json, t.invite_key, t.source_workspace_id, t.created_by, t.created_at, t.updated_at,
	a.theme_image, a.guide_title, a.guide_description, a.guide_enabled, a.recall_enabled, a.recall_status_id`

// issueInviteTemplateRow is the row shape shared by invitation-template queries.
type issueInviteTemplateRow struct {
	ID               int
	InviteKey        string
	TypeKey          string
	Name             string
	DescriptionMode  string
	FieldsJSON       string
	ThemeImage       string
	GuideTitle       string
	GuideDescription string
	GuideEnabled     int
	Enabled          int
	SortOrder        int
	RecallEnabled    int
	RecallStatusID   int
	CreatedBy        int
	CreatedAt        int64
	UpdatedAt        int64
}

// toIssueInviteTemplateRow converts one joined row into the shared row shape.
func toIssueInviteTemplateRow(row msql.Params) (issueInviteTemplateRow, error) {
	id, err := cast.ToIntE(row[`id`])
	if err != nil {
		return issueInviteTemplateRow{}, err
	}
	enabled := 0
	if raw, ok := row[`enabled`]; ok {
		enabled = cast.ToInt(raw)
	}
	sortOrder := 0
	if raw, ok := row[`sort_order`]; ok {
		sortOrder = cast.ToInt(raw)
	}
	// 老缓存里没有这一列时按开启处理，维持“配了文案就展示”的旧行为
	guideEnabled := 1
	if raw, ok := row[`guide_enabled`]; ok {
		guideEnabled = cast.ToInt(raw)
	}
	return issueInviteTemplateRow{
		ID: id, InviteKey: row[`invite_key`], TypeKey: row[`type_key`], Name: row[`name`],
		DescriptionMode: row[`description_mode`], FieldsJSON: row[`fields_json`],
		ThemeImage: row[`theme_image`], GuideTitle: row[`guide_title`],
		GuideDescription: row[`guide_description`],
		GuideEnabled:     guideEnabled,
		Enabled:          enabled, SortOrder: sortOrder,
		RecallEnabled: cast.ToInt(row[`recall_enabled`]), RecallStatusID: cast.ToInt(row[`recall_status_id`]),
		CreatedBy: cast.ToInt(row[`created_by`]), CreatedAt: cast.ToInt64(row[`created_at`]),
		UpdatedAt: cast.ToInt64(row[`updated_at`]),
	}, nil
}

// rowToIssueInviteTemplateModel converts one row into the public model.
func rowToIssueInviteTemplateModel(row issueInviteTemplateRow, adminID, workspaceID int) define.IssueInviteTemplateModel {
	return define.IssueInviteTemplateModel{
		ID: row.ID, AdminID: adminID, WorkspaceID: workspaceID,
		InviteKey: row.InviteKey, TypeKey: row.TypeKey, Name: row.Name,
		DescriptionMode: row.DescriptionMode, FieldsJSON: row.FieldsJSON,
		ThemeImage: row.ThemeImage, GuideTitle: row.GuideTitle, GuideDescription: row.GuideDescription,
		GuideEnabled: row.GuideEnabled, Enabled: row.Enabled, SortOrder: row.SortOrder, CreatedBy: row.CreatedBy,
		RecallEnabled: row.RecallEnabled, RecallStatusID: row.RecallStatusID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// nextIssueInviteTemplateSort returns the next sort_order inside the transaction.
func nextIssueInviteTemplateSort(tx *sql.Tx, adminID, workspaceID int) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT COALESCE(MAX(sort_order), -1) + 1 AS next_sort
		FROM `+define.TableWorkspaceIssueItemTemplates+` WHERE admin_id = $1 AND workspace_id = $2`, tx, adminID, workspaceID)
	if err != nil {
		logs.Error("next issue invite template sort failed, err:%s", err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return cast.ToInt(rows[0][`next_sort`]), nil
}

// issueInviteHiddenFieldKeys 是列表里固定不显示的字段：ID，以及公开表单永远不渲染的创建人、
// 创建时间、进度和三个工时派生字段（进度和工时是查询算出来的，本来就没有可填的列）。
// 开关在设置页是锁死的，保存在这里的提交值也一律改回不显示。
var issueInviteHiddenFieldKeys = map[string]bool{
	`id`: true, `created_at`: true, `created_by`: true,
	`work_progress`: true, `remaining_work_hours`: true, `overtime_work_hours`: true,
}

// issueInviteNoLinkParamKeys 是链接传参不生效的字段：ID 和状态由系统自己定。
var issueInviteNoLinkParamKeys = map[string]bool{`id`: true, `status`: true}

// issueInviteAlwaysShownFieldKeys 是字段列表里能看到但关不掉的字段：工单标题和工单内容是公开
// 表单固定的必填项，关掉就没有内容可提交了。
var issueInviteAlwaysShownFieldKeys = map[string]bool{`title`: true, `description`: true}

// issueInviteFixedDescription 是公开表单的工单内容。它不在属性字段里，但每次都要有，
// 老模板没存过它就由这里补上，默认排在标题后面，显示与必填都由这里钉死。
func issueInviteFixedDescription() TemplateFieldModel {
	shown := true
	return TemplateFieldModel{FieldKey: `description`, Required: true, Display: &shown}
}

// applyIssueInviteFieldLocks 把系统自己定的字段钉死，不接受管理员或提交值改动。
func applyIssueInviteFieldLocks(fields []TemplateFieldModel) {
	hidden := false
	shown := true
	for i := range fields {
		if issueInviteHiddenFieldKeys[fields[i].FieldKey] {
			fields[i].Display = &hidden
		}
		if issueInviteAlwaysShownFieldKeys[fields[i].FieldKey] {
			fields[i].Display = &shown
			// 工单标题和工单内容是公开表单固定的必填项，必填不受模板配置影响
			fields[i].Required = true
		}
		if issueInviteNoLinkParamKeys[fields[i].FieldKey] {
			fields[i].LinkParam = ``
		}
	}
}

// issueInviteEnabledFields lists the fields an invitation form can carry, in attribute-field
// order: every built-in field the project has switched on (including the read-only ones such
// as id and created_at, which the public form never draws), then the
// project's enabled custom fields. The form's content field sits right after the title.
func issueInviteEnabledFields(adminID, workspaceID int, typeKey string) ([]TemplateFieldModel, error) {
	wsID := cast.ToString(workspaceID)
	shown := true
	configs, err := ListFieldConfigs(ListFieldConfigsModel{AdminId: adminID, WsId: wsID, TypeKey: typeKey})
	if err != nil {
		return nil, err
	}
	fields := make([]TemplateFieldModel, 0, len(configs.Items)+1)
	descriptionPlaced := false
	for _, config := range configs.Items {
		if config.Enabled != 1 {
			continue
		}
		fields = append(fields, TemplateFieldModel{
			FieldKey: config.Key, Required: config.Key == `title`, Display: &shown,
		})
		if config.Key == `title` {
			fields = append(fields, issueInviteFixedDescription())
			descriptionPlaced = true
		}
	}
	// 标题被关掉时工单内容还是要填
	if !descriptionPlaced {
		fields = append([]TemplateFieldModel{issueInviteFixedDescription()}, fields...)
	}
	customs, err := ListCustomFields(ListCustomFieldsModel{AdminId: adminID, WsId: wsID, TypeKey: typeKey})
	if err != nil {
		return nil, err
	}
	for _, custom := range customs.Items {
		if custom.IsEnabled != 1 {
			continue
		}
		fields = append(fields, TemplateFieldModel{
			FieldKey: `custom:` + cast.ToString(custom.Id), FieldName: custom.Name,
			FieldType: custom.FieldType, Display: &shown,
		})
	}
	applyIssueInviteFieldLocks(fields)
	return fields, nil
}

// issueInviteDefaultDisplay 决定模板里没记录过的字段默认显不显示。
// 公开表单永远不画的内置字段（ID、创建人、创建时间、进度与工时派生）默认关着，
// 开着也只是个死开关；状态这类只读展示的内置字段默认开着；能提交的字段和自定义
// 字段仍默认关着，免得项目后开的字段自己跑到公开表单上。
func issueInviteDefaultDisplay(fieldKey string) *bool {
	shown := true
	hidden := false
	if templateCustomFieldID(fieldKey) > 0 {
		return &hidden
	}
	if issueInviteHiddenFieldKeys[fieldKey] {
		return &hidden
	}
	if _, submittable := issueInviteWritableBuiltinColumns[fieldKey]; submittable {
		return &hidden
	}
	return &shown
}

// resolveIssueInviteTemplateFields merges the stored toggles with the project's current
// enabled fields. The stored order wins — that is the order the operator dragged the rows
// into — and a field the template never stored (one the project added afterwards) is appended
// in attribute-field order with issueInviteDefaultDisplay, so a submittable field never leaks
// onto the public form on its own.
func resolveIssueInviteTemplateFields(adminID, workspaceID int, typeKey, fieldsJSON string) ([]TemplateFieldModel, error) {
	stored, err := decodeTemplateFields(fieldsJSON)
	if err != nil {
		return nil, err
	}
	enabled, err := issueInviteEnabledFields(adminID, workspaceID, typeKey)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]TemplateFieldModel, len(enabled))
	for _, field := range enabled {
		byKey[field.FieldKey] = field
	}
	seen := make(map[string]struct{}, len(enabled))
	result := make([]TemplateFieldModel, 0, len(enabled))
	for _, field := range stored {
		project, exists := byKey[field.FieldKey]
		if _, dup := seen[field.FieldKey]; dup || !exists {
			continue
		}
		project.Display = field.Display
		project.LinkParam = field.LinkParam
		// 必填跟着模板存的配置走，公开表单按它校验
		project.Required = field.Required
		result = append(result, project)
		seen[field.FieldKey] = struct{}{}
	}
	for _, field := range enabled {
		if _, done := seen[field.FieldKey]; done {
			continue
		}
		field.Display = issueInviteDefaultDisplay(field.FieldKey)
		result = append(result, field)
	}
	applyIssueInviteFieldLocks(result)
	return result, nil
}

// normalizeIssueInviteTemplateFields validates an invitation template's field list and returns
// the stored JSON. An invitation template owns no field data: each row only records whether the
// field shows on the public form and which link parameter fills it.
func normalizeIssueInviteTemplateFields(adminID, workspaceID int, typeKey, fieldsJSON string) (string, error) {
	submitted, err := decodeTemplateFields(fieldsJSON)
	if err != nil {
		return ``, err
	}
	context, err := loadTemplateFieldContext(adminID, cast.ToString(workspaceID), typeKey)
	if err != nil {
		return ``, err
	}
	seen := make(map[string]struct{}, len(submitted))
	seenParams := make(map[string]struct{}, len(submitted))
	fields := make([]TemplateFieldModel, 0, len(submitted)+1)
	descriptionPlaced := false
	for _, field := range submitted {
		key := strings.TrimSpace(field.FieldKey)
		if key == `` {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		if customID := templateCustomFieldID(key); customID > 0 {
			if _, exists := context.CustomByID[customID]; !exists {
				// 字段被删了，跟着丢掉
				continue
			}
		} else if key != `description` && !context.Builtins[key] {
			// 工单内容不在属性字段里，但它是公开表单固定的字段，照旧收下
			continue
		}
		linkParam := ``
		if !issueInviteNoLinkParamKeys[key] {
			linkParam, err = normalizeIssueInviteLinkParam(field.LinkParam, seenParams)
			if err != nil {
				return ``, err
			}
			if linkParam != `` {
				seenParams[linkParam] = struct{}{}
			}
		}
		seen[key] = struct{}{}
		fields = append(fields, TemplateFieldModel{
			FieldKey: key, Display: field.Display, LinkParam: linkParam,
			Required: field.Required,
		})
		if key == `description` {
			descriptionPlaced = true
		}
	}
	// 老模板没存过工单内容，补在标题后面
	if !descriptionPlaced {
		at := 0
		for i, field := range fields {
			if field.FieldKey == `title` {
				at = i + 1
				break
			}
		}
		merged := make([]TemplateFieldModel, 0, len(fields)+1)
		merged = append(merged, fields[:at]...)
		merged = append(merged, issueInviteFixedDescription())
		merged = append(merged, fields[at:]...)
		fields = merged
	}
	// 标题和描述是公开表单的必填项，提交值里不带 required
	for i := range fields {
		if fields[i].FieldKey == `title` || fields[i].FieldKey == `description` {
			fields[i].Required = true
		}
	}
	applyIssueInviteFieldLocks(fields)
	return encodeTemplateFields(fields)
}

// issueInviteLinkParamMaxLength 是链接参数名的长度上限，跟字段名一个量级。
const issueInviteLinkParamMaxLength = 64

// normalizeIssueInviteLinkParam 检查链接参数名：留空表示这个字段不参与链接传参；
// 填了就只能用字母数字和 _-，同一个模板里两个字段也不能共用同一个参数名。
func normalizeIssueInviteLinkParam(raw string, seen map[string]struct{}) (string, error) {
	param := strings.TrimSpace(raw)
	if param == `` {
		return ``, nil
	}
	if len(param) > issueInviteLinkParamMaxLength {
		return ``, ErrIssueInviteLinkParamInvalid
	}
	for _, char := range param {
		if !isIssueInviteLinkParamChar(char) {
			return ``, ErrIssueInviteLinkParamInvalid
		}
	}
	if _, dup := seen[param]; dup {
		return ``, ErrIssueInviteLinkParamInvalid
	}
	return param, nil
}

// isIssueInviteLinkParamChar 只放行 URL 查询串里不用转义的字符，参数名要能手写进链接。
func isIssueInviteLinkParamChar(char rune) bool {
	switch {
	case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		return true
	case char == '_', char == '-':
		return true
	}
	return false
}

// ListIssueInviteTemplates returns invitation templates in configured order.
func ListIssueInviteTemplates(adminID, workspaceID int, enabledOnly bool) ([]define.IssueInviteTemplateModel, error) {
	query := `SELECT ` + issueInviteTemplateFields + `, a.enabled, a.sort_order
		FROM ` + define.TableWorkspaceIssueItemTemplates + ` a
		JOIN ` + define.TableWorkItemTemplates + ` t
			ON t.admin_id = a.admin_id AND t.id = a.template_id
		WHERE a.admin_id = $1 AND a.workspace_id = $2`
	args := []any{adminID, workspaceID}
	if enabledOnly {
		query += ` AND a.enabled = 1`
	}
	query += ` ORDER BY a.sort_order ASC, a.id ASC`
	rows, err := msql.RawValues(define.Postgres, query, nil, args...)
	if err != nil {
		logs.Error("list issue invite templates failed, err:%s", err.Error())
		return nil, err
	}
	items := make([]define.IssueInviteTemplateModel, 0, len(rows))
	for _, row := range rows {
		converted, convErr := toIssueInviteTemplateRow(row)
		if convErr != nil {
			return nil, convErr
		}
		items = append(items, rowToIssueInviteTemplateModel(converted, adminID, workspaceID))
	}
	return items, nil
}

// getIssueInviteTemplateRow returns one template in the authenticated project scope.
func getIssueInviteTemplateRow(adminID, workspaceID, templateID int) (issueInviteTemplateRow, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT `+issueInviteTemplateFields+`, a.enabled, a.sort_order
		FROM `+define.TableWorkspaceIssueItemTemplates+` a
		JOIN `+define.TableWorkItemTemplates+` t
			ON t.admin_id = a.admin_id AND t.id = a.template_id
		WHERE a.admin_id = $1 AND a.workspace_id = $2 AND a.template_id = $3`, nil, adminID, workspaceID, templateID)
	if err != nil {
		logs.Error("get issue invite template failed, err:%s", err.Error())
		return issueInviteTemplateRow{}, err
	}
	if len(rows) == 0 {
		return issueInviteTemplateRow{}, ErrIssueInviteTemplateNotFound
	}
	return toIssueInviteTemplateRow(rows[0])
}

// issueInviteTemplateInsertModel carries one invitation-template insert inside a transaction.
type issueInviteTemplateInsertModel struct {
	Tx               *sql.Tx
	AdminID          int
	WorkspaceID      int
	TypeKey          string
	Name             string
	DescriptionMode  string
	FieldsJSON       string
	ThemeImage       string
	GuideTitle       string
	GuideDescription string
	GuideEnabled     int
	Enabled          int
	SortOrder        int
	RecallEnabled    int
	RecallStatusID   int
	OperatorID       int
	Now              int64
}

// insertIssueInviteTemplateTx inserts the template row plus its project association row.
// The public key is generated here; a collision inserts nothing and just draws another key.
func insertIssueInviteTemplateTx(p issueInviteTemplateInsertModel) (int, error) {
	for i := 0; i < define.IssueInviteKeyRetryMaxCount; i++ {
		inviteKey, genErr := generateIssueInviteKey()
		if genErr != nil {
			return 0, genErr
		}
		inserted, insertErr := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkItemTemplates+`
			(admin_id, source_workspace_id, type_key, name, description_mode, fields_json, invite_key, created_by, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
			ON CONFLICT (invite_key) WHERE invite_key <> '' DO NOTHING
			RETURNING id`, p.Tx,
			p.AdminID, p.WorkspaceID, p.TypeKey, p.Name,
			p.DescriptionMode, p.FieldsJSON, inviteKey, p.OperatorID, p.Now)
		if insertErr != nil {
			logs.Error("insert issue invite template failed, err:%s", insertErr.Error())
			return 0, insertErr
		}
		if len(inserted) == 0 {
			continue
		}
		templateID := cast.ToInt(inserted[0][`id`])
		if _, assocErr := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceIssueItemTemplates+`
			(admin_id, workspace_id, template_id, enabled, sort_order, theme_image, guide_title, guide_description,
				guide_enabled, recall_enabled, recall_status_id, created_by, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)`, p.Tx,
			p.AdminID, p.WorkspaceID, templateID, p.Enabled, p.SortOrder,
			p.ThemeImage, p.GuideTitle, p.GuideDescription, p.GuideEnabled,
			p.RecallEnabled, p.RecallStatusID, p.OperatorID, p.Now); assocErr != nil {
			logs.Error("insert issue invite template association failed, err:%s", assocErr.Error())
			return 0, assocErr
		}
		return templateID, nil
	}
	return 0, ErrIssueInviteKeyGenerate
}

// CreateIssueInviteTemplate inserts one independent invitation template together with
// its project association row; the public key is generated here.
func CreateIssueInviteTemplate(model define.IssueInviteTemplateCreateModel) (define.IssueInviteTemplateModel, error) {
	fieldsJSON, err := normalizeIssueInviteTemplateFields(model.AdminID, model.WorkspaceID, model.TypeKey, model.FieldsJSON)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	mode := model.DescriptionMode
	if mode == `` {
		mode = define.DescriptionModeRichText
	}
	recallStatusID, err := resolveIssueInviteRecallStatus(model.AdminID, model.WorkspaceID, model.TypeKey, model.RecallEnabled, model.RecallStatusID)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	now := int64(tool.Time2Int())
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	sortOrder, err := nextIssueInviteTemplateSort(tx, model.AdminID, model.WorkspaceID)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	templateID, err := insertIssueInviteTemplateTx(issueInviteTemplateInsertModel{
		Tx: tx, AdminID: model.AdminID, WorkspaceID: model.WorkspaceID, TypeKey: model.TypeKey,
		Name: model.Name, DescriptionMode: mode,
		FieldsJSON: fieldsJSON, ThemeImage: model.ThemeImage, GuideTitle: model.GuideTitle,
		GuideDescription: model.GuideDescription, GuideEnabled: model.GuideEnabled,
		Enabled: model.Enabled, SortOrder: sortOrder,
		RecallEnabled: model.RecallEnabled, RecallStatusID: recallStatusID,
		OperatorID: model.OperatorID, Now: now,
	})
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit issue invite template create failed, err:%s", err.Error())
		return define.IssueInviteTemplateModel{}, err
	}
	if err = InvalidateIssueInviteProjectCache(model.AdminID, model.WorkspaceID); err != nil {
		logs.Error("invalidate issue invite cache after create failed, err:%s", err.Error())
	}
	row, err := getIssueInviteTemplateRow(model.AdminID, model.WorkspaceID, templateID)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	return resolveIssueInviteTemplateModelFields(rowToIssueInviteTemplateModel(row, model.AdminID, model.WorkspaceID))
}

// GetIssueInviteTemplate returns one template in the authenticated project scope, with its
// field list merged against the project's current enabled fields.
func GetIssueInviteTemplate(adminID, workspaceID, templateID int) (define.IssueInviteTemplateModel, error) {
	row, err := getIssueInviteTemplateRow(adminID, workspaceID, templateID)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	model := rowToIssueInviteTemplateModel(row, adminID, workspaceID)
	return resolveIssueInviteTemplateModelFields(model)
}

// resolveIssueInviteTemplateModelFields rewrites one template's FieldsJSON into the merged
// field list the editor and the public form both read.
func resolveIssueInviteTemplateModelFields(model define.IssueInviteTemplateModel) (define.IssueInviteTemplateModel, error) {
	fields, err := resolveIssueInviteTemplateFields(model.AdminID, model.WorkspaceID, model.TypeKey, model.FieldsJSON)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	encoded, err := encodeTemplateFields(fields)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	model.FieldsJSON = encoded
	return model, nil
}

// resolveIssueInviteRecallStatus 校验撤回后的状态属于该项目的该类型。传 0 时按「已关闭」补默认值，
// 项目改过状态名就补不上，此时开着撤回会报错让管理员自己选。
func resolveIssueInviteRecallStatus(adminID, workspaceID int, typeKey string, enabled, statusID int) (int, error) {
	resolved := 0
	if statusID > 0 {
		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableWorkItemStatuses+`
			WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND id = $4`, nil,
			adminID, workspaceID, typeKey, statusID)
		if err != nil {
			logs.Error("check issue invite recall status failed, err:%s", err.Error())
			return 0, err
		}
		if len(rows) == 0 {
			return 0, ErrIssueInviteRecallStatusInvalid
		}
		resolved = statusID
	} else {
		// 内置状态的 name 存的是 i18n key 包装串，按它找「已关闭」
		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableWorkItemStatuses+`
			WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND name = $4`, nil,
			adminID, workspaceID, typeKey, lib_i18n.WrapKey(`workitem_status_closed`))
		if err != nil {
			logs.Error("load default issue invite recall status failed, err:%s", err.Error())
			return 0, err
		}
		if len(rows) > 0 {
			resolved = cast.ToInt(rows[0][`id`])
		}
	}
	if enabled == 1 && resolved == 0 {
		return 0, ErrIssueInviteRecallStatusInvalid
	}
	return resolved, nil
}

// UpdateIssueInviteTemplate updates one invitation template.
func UpdateIssueInviteTemplate(model define.IssueInviteTemplateUpdateModel) (define.IssueInviteTemplateModel, error) {
	existing, err := getIssueInviteTemplateRow(model.AdminID, model.WorkspaceID, model.TemplateID)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	fieldsJSON, err := normalizeIssueInviteTemplateFields(model.AdminID, model.WorkspaceID, existing.TypeKey, model.FieldsJSON)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}

	mode := model.DescriptionMode
	if mode == `` {
		mode = define.DescriptionModeRichText
	}
	recallStatusID, err := resolveIssueInviteRecallStatus(model.AdminID, model.WorkspaceID, existing.TypeKey, model.RecallEnabled, model.RecallStatusID)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	now := tool.Time2Int()
	m := msql.Model(define.TableWorkItemTemplates, define.Postgres)
	n, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminID)},
		{`id`, cast.ToString(model.TemplateID)},
	}).Update(msql.Datas{
		`name`:             model.Name,
		`description_mode`: mode,
		`fields_json`:      fieldsJSON,
		`updated_at`:       now,
	})
	if err != nil {
		logs.Error("update issue invite template failed, sql:%s, err:%s", m.GetLastSql(), err.Error())
		return define.IssueInviteTemplateModel{}, err
	}
	if n == 0 {
		return define.IssueInviteTemplateModel{}, ErrIssueInviteTemplateNotFound
	}

	// The enabled toggle and template-level theme/guide fields live on the association row.
	a := msql.Model(define.TableWorkspaceIssueItemTemplates, define.Postgres)
	if _, err = a.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminID)},
		{`workspace_id`, cast.ToString(model.WorkspaceID)},
		{`template_id`, cast.ToString(model.TemplateID)},
	}).Update(msql.Datas{
		`enabled`:           model.Enabled,
		`theme_image`:       model.ThemeImage,
		`guide_title`:       model.GuideTitle,
		`guide_description`: model.GuideDescription,
		`guide_enabled`:     model.GuideEnabled,
		`recall_enabled`:    model.RecallEnabled,
		`recall_status_id`:  recallStatusID,
		`updated_at`:        now,
	}); err != nil {
		logs.Error("update issue invite template association failed, sql:%s, err:%s", a.GetLastSql(), err.Error())
		return define.IssueInviteTemplateModel{}, err
	}

	if err = InvalidateIssueInviteProjectCache(model.AdminID, model.WorkspaceID); err != nil {
		logs.Error("invalidate issue invite cache after template update failed, err:%s", err.Error())
	}
	if err = RotateIssueInviteRenderVersion(model.AdminID, model.WorkspaceID); err != nil {
		logs.Error("rotate issue invite render version after template update failed, err:%s", err.Error())
	}

	row, err := getIssueInviteTemplateRow(model.AdminID, model.WorkspaceID, model.TemplateID)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	return resolveIssueInviteTemplateModelFields(rowToIssueInviteTemplateModel(row, model.AdminID, model.WorkspaceID))
}

// ReorderIssueInviteTemplates replaces the complete project template order atomically.
func ReorderIssueInviteTemplates(model define.IssueInviteTemplateOrderModel) error {
	if err := validate.IssueInviteTemplateOrder(model); err != nil {
		return ErrIssueInviteOrderInvalid
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	currentRows, err := msql.RawValues(define.Postgres, `SELECT template_id FROM `+define.TableWorkspaceIssueItemTemplates+`
		WHERE admin_id = $1 AND workspace_id = $2 FOR UPDATE`, tx, model.AdminID, model.WorkspaceID)
	if err != nil {
		logs.Error("lock issue invite template order failed, err:%s", err.Error())
		return err
	}
	current := make(map[int]struct{}, len(currentRows))
	for _, row := range currentRows {
		current[cast.ToInt(row[`template_id`])] = struct{}{}
	}
	if len(current) != len(model.OrderedTemplateID) {
		return ErrIssueInviteOrderInvalid
	}
	for _, id := range model.OrderedTemplateID {
		if _, ok := current[id]; !ok {
			return ErrIssueInviteOrderInvalid
		}
	}
	now := tool.Time2Int()
	for sortOrder, templateID := range model.OrderedTemplateID {
		if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceIssueItemTemplates+`
			SET sort_order = $1, updated_at = $2
			WHERE admin_id = $3 AND workspace_id = $4 AND template_id = $5`, tx,
			sortOrder, now, model.AdminID, model.WorkspaceID, templateID); err != nil {
			logs.Error("update issue invite template order failed, err:%s", err.Error())
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit issue invite template order failed, err:%s", err.Error())
		return err
	}
	return InvalidateIssueInviteProjectCache(model.AdminID, model.WorkspaceID)
}

// RemoveIssueInviteTemplate removes one template without deleting historical issues.
func RemoveIssueInviteTemplate(adminID, workspaceID, templateID, operatorID int) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	assocRows, err := msql.RawValues(define.Postgres, `SELECT a.template_id, t.invite_key
		FROM `+define.TableWorkspaceIssueItemTemplates+` a
		JOIN `+define.TableWorkItemTemplates+` t
			ON t.admin_id = a.admin_id AND t.id = a.template_id
		WHERE a.admin_id = $1 AND a.workspace_id = $2 AND a.template_id = $3
		FOR UPDATE OF a`, tx, adminID, workspaceID, templateID)
	if err != nil {
		logs.Error("lock issue invite template removal failed, err:%s", err.Error())
		return err
	}
	if len(assocRows) == 0 {
		return ErrIssueInviteTemplateNotFound
	}
	removedKey := assocRows[0][`invite_key`]

	// Delete the association first, then the template row itself.
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableWorkspaceIssueItemTemplates+`
		WHERE admin_id = $1 AND workspace_id = $2 AND template_id = $3`, tx, adminID, workspaceID, templateID); err != nil {
		logs.Error("delete issue invite template association failed, err:%s", err.Error())
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableWorkItemTemplates+`
		WHERE admin_id = $1 AND id = $2 AND source_workspace_id = $3 AND invite_key <> ''`, tx, adminID, templateID, workspaceID); err != nil {
		logs.Error("delete issue invite template failed, err:%s", err.Error())
		return err
	}

	if err = tx.Commit(); err != nil {
		logs.Error("commit issue invite template removal failed, err:%s", err.Error())
		return err
	}
	// The removed template is gone from the project list, so its own public cache key
	// must be deleted explicitly; the project-wide invalidation cannot discover it.
	if err = InvalidateIssueInviteTemplateCache(adminID, workspaceID, removedKey); err != nil {
		logs.Error("invalidate removed issue invite template cache failed, err:%s", err.Error())
	}
	// Historical issues of a removed template stay readable; their cached detail would still
	// offer 撤回 against a template that no longer exists.
	if err = RotateIssueInviteRenderVersion(adminID, workspaceID); err != nil {
		logs.Error("rotate issue invite render version after template removal failed, err:%s", err.Error())
	}
	return InvalidateIssueInviteProjectCache(adminID, workspaceID)
}
