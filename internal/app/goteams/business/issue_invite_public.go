// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"context"
	"database/sql"
	"errors"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// issueInviteHTMLTagPattern matches one HTML tag while stripping comment markup.
var issueInviteHTMLTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

// issueInvitePlainText flattens an internal comment to plain text before it is exposed on
// the public page: the submitter must not see internal markup or internal media URLs.
func issueInvitePlainText(content string) string {
	plain := issueInviteHTMLTagPattern.ReplaceAllString(content, " ")
	plain = html.UnescapeString(plain)
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(plain, " "))
}

// ResolvePublicIssueInvite locates a public key and re-verifies its enabled project scope.
func ResolvePublicIssueInvite(inviteKey string) (define.IssueInviteConfigModel, error) {
	if err := validate.IssueInviteKey(inviteKey); err != nil {
		return define.IssueInviteConfigModel{}, ErrIssueInviteInvalid
	}
	// The globally unique key is the only locator exception; afterwards every lookup is re-scoped.
	m := msql.Model(define.TableWorkspaceIssueInvites, define.Postgres)
	row, err := m.Where2([][]string{{`invite_key`, inviteKey}}).
		Field(`admin_id, workspace_id, invite_key, guide_title, guide_description, theme_image, created_by, created_at, updated_at`).Find()
	if err != nil {
		logs.Error("resolve public issue invite failed, sql:%s, err:%s", m.GetLastSql(), err.Error())
		return define.IssueInviteConfigModel{}, err
	}
	if len(row) == 0 {
		return define.IssueInviteConfigModel{}, ErrIssueInviteInvalid
	}
	adminID := cast.ToInt(row[`admin_id`])
	workspaceID := cast.ToInt(row[`workspace_id`])
	if adminID <= 0 || workspaceID <= 0 {
		return define.IssueInviteConfigModel{}, ErrIssueInviteInvalid
	}

	// Re-verify the app is enabled within the resolved project scope.
	app, err := GetWorkspaceApplication(adminID, workspaceID, define.AppKeyIssue)
	if err != nil {
		return define.IssueInviteConfigModel{}, err
	}
	if app.Enabled != 1 {
		return define.IssueInviteConfigModel{}, ErrIssueInviteInvalid
	}
	config, err := GetIssueInviteConfig(adminID, workspaceID)
	if err != nil {
		return define.IssueInviteConfigModel{}, err
	}
	if config.InviteKey != inviteKey {
		return define.IssueInviteConfigModel{}, ErrIssueInviteInvalid
	}
	return config, nil
}

// GetPublicIssueInvite returns enabled template summaries for a general invitation link.
func GetPublicIssueInvite(inviteKey, openID string) (define.IssueInviteConfigModel, []define.IssueInviteTemplateModel, error) {
	if err := validate.IssueInviteOpenID(openID); err != nil {
		return define.IssueInviteConfigModel{}, nil, ErrIssueInviteInvalid
	}
	scope, err := ResolvePublicIssueInvite(inviteKey)
	if err != nil {
		return define.IssueInviteConfigModel{}, nil, err
	}
	return GetCachedIssueInviteConfig(scope)
}

// GetPublicIssueTemplate returns one enabled copied template for a template link.
// lang carries the submitter's language so builtin option labels (e.g. priority) are translated.
func GetPublicIssueTemplate(inviteKey, templateKey, openID, lang string) (define.IssueInviteTemplateModel, error) {
	if err := validate.IssueInviteKey(templateKey); err != nil {
		return define.IssueInviteTemplateModel{}, ErrIssueInviteInvalid
	}
	if err := validate.IssueInviteOpenID(openID); err != nil {
		return define.IssueInviteTemplateModel{}, ErrIssueInviteInvalid
	}
	scope, err := ResolvePublicIssueInvite(inviteKey)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	template, err := GetCachedIssueInviteTemplate(scope, templateKey)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	return enrichIssueInviteTemplateOptions(template, scope.AdminID, scope.WorkspaceID, lang)
}

// issueInviteWritableBuiltinColumns maps a builtin field to the requirement column a public
// submitter may fill. Fields outside this map stay admin-controlled: the title comes from the
// template and the rest never render on the public form.
var issueInviteWritableBuiltinColumns = map[string]string{
	`status`:               `status_id`,
	`category`:             `category_id`,
	`priority`:             `priority`,
	`version`:              `version_id`,
	`planned_start_date`:   `planned_start_date`,
	`planned_end_date`:     `planned_end_date`,
	`assignee`:             `assignee_ids`,
	`estimated_work_hours`: `work_hours`,
	`completed_work_hours`: `completed_work_hours`,
}

// issueInviteSubmitEditableExempt 放行属性字段里标着不可编辑、但公开表单要收的字段：
// 状态在属性字段里是只读配置（is_editable=0），需求 2621 要求公开表单能选状态。
var issueInviteSubmitEditableExempt = map[string]bool{`status`: true}

// loadIssueInviteWritableBuiltins narrows the map above to the fields this project currently
// allows a submitter to fill: the attribute-field switch is on and the field is editable.
// A template showing such a field is what makes it submittable.
func loadIssueInviteWritableBuiltins(adminID int, wsID, typeKey string) (map[string]string, error) {
	configs, err := ListFieldConfigs(ListFieldConfigsModel{AdminId: adminID, WsId: wsID, TypeKey: typeKey})
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(issueInviteWritableBuiltinColumns))
	for _, config := range configs.Items {
		column, known := issueInviteWritableBuiltinColumns[config.Key]
		if !known || config.Enabled != 1 {
			continue
		}
		if config.Editable != 1 && !issueInviteSubmitEditableExempt[config.Key] {
			continue
		}
		result[config.Key] = column
	}
	return result, nil
}

// issueInviteStatusOptions returns the project's status list as public option pairs, names
// already translated to the request language.
func issueInviteStatusOptions(adminID int, wsID, typeKey, lang string) ([]map[string]any, error) {
	list, err := ListStatuses(ListStatusesModel{AdminId: adminID, WsId: wsID, TypeKey: typeKey, Lang: lang})
	if err != nil {
		return nil, err
	}
	options := make([]map[string]any, 0, len(list.Items))
	for _, item := range list.Items {
		options = append(options, map[string]any{`key`: item.Id, `name`: item.Name, `color`: item.Color})
	}
	return options, nil
}

// issueInvitePriorityOptions returns the project's priority list as public option pairs.
func issueInvitePriorityOptions(adminID int, wsID, typeKey, lang string) ([]map[string]any, error) {
	list, err := ListPriorities(ListPrioritiesModel{AdminId: adminID, WsId: wsID, TypeKey: typeKey, Lang: lang})
	if err != nil {
		return nil, err
	}
	options := make([]map[string]any, 0, len(list.Items))
	for _, item := range list.Items {
		options = append(options, map[string]any{`key`: item.Id, `name`: item.Label, `color`: item.Color})
	}
	return options, nil
}

// issueInviteAssigneeOptions returns active project members as public option pairs.
// Soft-deleted members are omitted so nobody can be newly assigned to a removed account.
func issueInviteAssigneeOptions(adminID int, wsID string) ([]map[string]any, error) {
	list, err := ListMembersByWorkspace(ListWorkspaceMembersModel{AdminId: adminID, WorkspaceId: wsID})
	if err != nil {
		return nil, err
	}
	options := make([]map[string]any, 0, len(list.Items))
	for _, item := range list.Items {
		if item.Removed || item.UserId <= 0 {
			continue
		}
		options = append(options, map[string]any{`key`: item.UserId, `name`: item.UserName, `color`: ``})
	}
	return options, nil
}

// issueInviteVersionOptions returns the project's iterations as public option pairs.
func issueInviteVersionOptions(adminID, wsID int) ([]map[string]any, error) {
	list, err := ListVersionsByWorkspace(adminID, wsID)
	if err != nil {
		return nil, err
	}
	options := make([]map[string]any, 0, len(list))
	for _, item := range list {
		options = append(options, map[string]any{`key`: item.Id, `name`: item.Name, `color`: ``})
	}
	return options, nil
}

// issueInviteCategoryOptions returns the project's requirement categories as public option pairs.
func issueInviteCategoryOptions(adminID, wsID int) ([]map[string]any, error) {
	list, err := ListRequirementCategories(ListRequirementCategoriesModel{AdminId: adminID, WorkspaceId: wsID})
	if err != nil {
		return nil, err
	}
	options := make([]map[string]any, 0, len(list.Items))
	for _, item := range list.Items {
		options = append(options, map[string]any{`key`: item.Id, `name`: item.Name, `color`: ``})
	}
	return options, nil
}

// issueInviteBuiltinOptions returns the option list of one submittable builtin field. Fields
// without a list (dates, hours) return nil so the response carries no options key.
func issueInviteBuiltinOptions(adminID, wsID int, typeKey, lang, key string) ([]map[string]any, error) {
	switch key {
	case `status`:
		return issueInviteStatusOptions(adminID, cast.ToString(wsID), typeKey, lang)
	case `priority`:
		return issueInvitePriorityOptions(adminID, cast.ToString(wsID), typeKey, lang)
	case `assignee`:
		return issueInviteAssigneeOptions(adminID, cast.ToString(wsID))
	case `version`:
		return issueInviteVersionOptions(adminID, wsID)
	case `category`:
		return issueInviteCategoryOptions(adminID, wsID)
	}
	return nil, nil
}

// enrichIssueInviteTemplateOptions attaches the current option lists to select-type custom
// fields and submittable builtin fields of a public template response; the cached copy
// itself stays untouched.
func enrichIssueInviteTemplateOptions(template define.IssueInviteTemplateModel, adminID, workspaceID int, lang string) (define.IssueInviteTemplateModel, error) {
	if strings.TrimSpace(template.FieldsJSON) == `` {
		return template, nil
	}
	resolved, err := resolveIssueInviteTemplateModelFields(template)
	if err != nil {
		return template, err
	}
	template = resolved
	var fields []map[string]any
	if err := tool.JsonDecodeUseNumber(template.FieldsJSON, &fields); err != nil {
		return template, err
	}
	customFields, err := ListCustomFields(ListCustomFieldsModel{AdminId: adminID, WsId: cast.ToString(workspaceID), TypeKey: template.TypeKey})
	if err != nil {
		return template, err
	}
	byID := make(map[int]CustomFieldModel, len(customFields.Items))
	for _, field := range customFields.Items {
		byID[field.Id] = field
	}
	wsID := cast.ToString(workspaceID)
	writable, err := loadIssueInviteWritableBuiltins(adminID, wsID, template.TypeKey)
	if err != nil {
		return template, err
	}
	// Builtin option lists are fetched lazily: most templates expose at most one of them.
	builtinOptions := make(map[string][]map[string]any, len(writable))
	changed := false
	for _, entry := range fields {
		key, _ := entry[`field_key`].(string)
		id := templateCustomFieldID(key)

		// Builtin submittable fields are resolved from project data.
		if id <= 0 {
			if _, submittable := writable[key]; !submittable {
				continue
			}
			options, exists := builtinOptions[key]
			if !exists {
				options, err = issueInviteBuiltinOptions(adminID, workspaceID, template.TypeKey, lang, key)
				if err != nil {
					return template, err
				}
				builtinOptions[key] = options
			}
			if len(options) == 0 {
				continue
			}
			entry[`options`] = options
			// 新建工单默认落在项目初始状态，表单打开就替提交人选中它
			if key == `status` {
				if initial, idErr := getInitialStatusId(adminID, wsID, template.TypeKey); idErr == nil && initial > 0 {
					entry[`default_value`] = initial
				}
			}
			changed = true
			continue
		}

		definition, ok := byID[id]
		if !ok || (definition.FieldType != `select_single` && definition.FieldType != `select_multi`) {
			continue
		}
		options := make([]map[string]any, 0, len(definition.Options))
		for _, option := range definition.Options {
			options = append(options, map[string]any{`key`: option.Key, `name`: option.Name, `color`: option.Color})
		}
		entry[`options`] = options
		changed = true
	}
	if !changed {
		return template, nil
	}
	encoded, err := tool.JsonEncode(fields)
	if err != nil {
		return template, err
	}
	template.FieldsJSON = encoded
	return template, nil
}

// submitIssueInviteFieldKey maps a public field_key to a custom field id; zero means builtin.
func submitIssueInviteFieldKey(key string) int {
	return templateCustomFieldID(key)
}

// issueInviteSubmission is the normalized result of one public submission.
type issueInviteSubmission struct {
	Title        string
	Description  string
	CustomValues map[string]any
	// Builtins the submitter explicitly chose, keyed by field_key. A missing key means
	// "not submitted", so the caller keeps the project default instead of writing an
	// empty column.
	Builtins map[string]any
	// DeclaredCustom 是模板里显示的自定义字段 id，重新提交时按这批 id 整份覆盖旧值。
	DeclaredCustom []int
}

// buildIssueInviteSubmission normalizes public fields against the invitation schema and returns
// the work-item title, description, custom field values keyed by custom field id and the
// builtin values the submitter chose.
func buildIssueInviteSubmission(template define.IssueInviteTemplateModel, rawFields []define.IssueInviteFieldValue) (issueInviteSubmission, error) {
	result := issueInviteSubmission{CustomValues: map[string]any{}, Builtins: map[string]any{}}
	wsID := cast.ToString(template.WorkspaceID)
	fields, err := resolveIssueInviteTemplateFields(template.AdminID, template.WorkspaceID, template.TypeKey, template.FieldsJSON)
	if err != nil {
		return issueInviteSubmission{}, err
	}
	writable, err := loadIssueInviteWritableBuiltins(template.AdminID, wsID, template.TypeKey)
	if err != nil {
		return issueInviteSubmission{}, err
	}

	// Declared custom fields from the template; only these may be submitted.
	// Hidden fields (display=false) are excluded so the external form cannot
	// submit them and their required flag never blocks the submitter.
	declaredCustom := make(map[int]struct{}, len(fields))
	requiredCustom := make(map[int]bool, len(fields))
	customNames := make(map[int]string, len(fields))
	declaredBuiltin := make(map[string]struct{}, len(writable))
	for _, field := range fields {
		if field.Display != nil && !*field.Display {
			continue
		}
		if id := templateCustomFieldID(field.FieldKey); id > 0 {
			declaredCustom[id] = struct{}{}
			requiredCustom[id] = field.Required
			customNames[id] = field.FieldName
			continue
		}
		// 模板里显示开启、属性字段又允许编辑的内置字段，公开表单才收
		if _, submittable := writable[field.FieldKey]; submittable {
			declaredBuiltin[field.FieldKey] = struct{}{}
		}
	}

	submitted := make(map[string]any, len(rawFields))
	for _, field := range rawFields {
		key := strings.TrimSpace(field.FieldKey)
		if _, dup := submitted[key]; dup {
			return issueInviteSubmission{}, ErrIssueInviteFieldInvalid
		}
		submitted[key] = field.Value
	}

	// 标题和内容只认提交值，模板不再存默认文案
	title := ``
	if value, ok := submitted[`title`]; ok {
		text, convErr := customFieldValueText(value)
		if convErr != nil {
			return issueInviteSubmission{}, convErr
		}
		title = strings.TrimSpace(text)
	}
	description := ``
	if value, ok := submitted[`description`]; ok {
		text, convErr := customFieldValueText(value)
		if convErr != nil {
			return issueInviteSubmission{}, convErr
		}
		description = text
	}
	// 标题和描述是公开表单固定的必填项，其余字段都由模板的显示开关决定
	if title == `` || strings.TrimSpace(description) == `` {
		return issueInviteSubmission{}, ErrIssueInviteFieldInvalid
	}

	customValues := make(map[string]any, len(declaredCustom))
	for key, value := range submitted {
		id := submitIssueInviteFieldKey(key)
		if id <= 0 {
			switch key {
			case `title`, `description`:
				continue
			}
			if _, declared := declaredBuiltin[key]; !declared {
				return issueInviteSubmission{}, ErrIssueInviteFieldInvalid
			}
			if err := applyIssueInviteBuiltinValue(&result, key, value); err != nil {
				return issueInviteSubmission{}, err
			}
			continue
		}
		if _, declared := declaredCustom[id]; !declared {
			return issueInviteSubmission{}, ErrIssueInviteFieldInvalid
		}
		customValues[cast.ToString(id)] = value
	}
	for id, required := range requiredCustom {
		if !required {
			continue
		}
		value, exists := customValues[cast.ToString(id)]
		text, convErr := customFieldValueText(value)
		if convErr != nil {
			return issueInviteSubmission{}, convErr
		}
		if !exists || text == `` || text == `[]` {
			// 字段名来自模板配置；拿不到名字时退回通用提示，不裸出字段 key
			if name := customNames[id]; name != `` {
				return issueInviteSubmission{}, newCustomFieldValidationError(
					ErrCustomFieldValueInvalid, define.MsgCustomFieldRequired, name)
			}
			return issueInviteSubmission{}, ErrIssueInviteFieldInvalid
		}
	}
	result.Title = title
	result.Description = description
	result.CustomValues = customValues
	result.DeclaredCustom = make([]int, 0, len(declaredCustom))
	for id := range declaredCustom {
		result.DeclaredCustom = append(result.DeclaredCustom, id)
	}
	return result, nil
}

// applyIssueInviteBuiltinValue parses one submitted builtin value into the submission.
// IDs are only shape-checked here; SubmitPublicIssue re-validates them against the project
// scope before any write, so a forged id can never point at another tenant's data.
func applyIssueInviteBuiltinValue(result *issueInviteSubmission, key string, value any) error {
	switch key {
	case `status`, `category`, `priority`, `version`:
		id, err := customFieldValueInt(value)
		if err != nil {
			return err
		}
		result.Builtins[key] = id
	case `assignee`:
		ids, err := customFieldIntList(value)
		if err != nil {
			return err
		}
		result.Builtins[key] = ids
	case `planned_start_date`, `planned_end_date`:
		timestamp, err := issueInviteDateTimestamp(value)
		if err != nil {
			return err
		}
		result.Builtins[key] = timestamp
	case `estimated_work_hours`, `completed_work_hours`:
		hours, err := issueInviteWorkHours(value)
		if err != nil {
			return err
		}
		result.Builtins[key] = hours
	default:
		return ErrIssueInviteFieldInvalid
	}
	return nil
}

// customFieldValueInt reads a positive option id from one submitted value.
func customFieldValueInt(value any) (int, error) {
	text, err := customFieldValueText(value)
	if err != nil {
		return 0, err
	}
	id := cast.ToInt(strings.TrimSpace(text))
	if id <= 0 {
		return 0, ErrIssueInviteFieldInvalid
	}
	return id, nil
}

// issueInviteDateTimestamp reads a planned date. The public form may send a Unix timestamp or a
// `2006-01-02` date; both are stored as a timestamp, and an empty value means "not filled".
func issueInviteDateTimestamp(value any) (int64, error) {
	text, err := customFieldValueText(value)
	if err != nil {
		return 0, err
	}
	text = strings.TrimSpace(text)
	if text == `` {
		return 0, nil
	}
	if timestamp := cast.ToInt64(text); timestamp > 0 {
		return timestamp, nil
	}
	parsed, parseErr := time.ParseInLocation(`2006-01-02`, text, time.Local)
	if parseErr != nil {
		return 0, ErrIssueInviteFieldInvalid
	}
	return parsed.Unix(), nil
}

// issueInviteWorkHours reads an hour count; an empty value means "not filled".
func issueInviteWorkHours(value any) (float64, error) {
	text, err := customFieldValueText(value)
	if err != nil {
		return 0, err
	}
	text = strings.TrimSpace(text)
	if text == `` {
		return 0, nil
	}
	hours, parseErr := strconv.ParseFloat(text, 64)
	if parseErr != nil || hours < 0 {
		return 0, ErrIssueInviteFieldInvalid
	}
	return hours, nil
}

// customFieldIntList normalizes a submitted value into a positive int list. It accepts a
// JSON number, a numeric string, or an array of either; empty input yields an empty list.
func customFieldIntList(value any) ([]int, error) {
	switch typed := value.(type) {
	case []any:
		ids := make([]int, 0, len(typed))
		for _, item := range typed {
			id := cast.ToInt(cast.ToString(item))
			if id <= 0 {
				return nil, ErrIssueInviteFieldInvalid
			}
			ids = append(ids, id)
		}
		return ids, nil
	case []int:
		return typed, nil
	case nil:
		return nil, nil
	default:
		text := strings.TrimSpace(cast.ToString(value))
		if text == `` {
			return nil, nil
		}
		id := cast.ToInt(text)
		if id <= 0 {
			return nil, ErrIssueInviteFieldInvalid
		}
		return []int{id}, nil
	}
}

// issueInviteBuiltinScopeTables maps a submittable option field to the table holding its ids.
// Status and priority are type-scoped; iterations and categories are shared by the whole project.
var issueInviteBuiltinScopeTables = map[string]string{
	`status`:   define.TableWorkItemStatuses,
	`priority`: define.TableWorkItemPriorities,
	`version`:  define.TableVersions,
	`category`: define.TableRequirementCategories,
}

// validateIssueInviteBuiltinScope re-checks the submitter-chosen builtins against the resolved
// project scope. The public endpoint is unauthenticated, so every id must belong to this
// admin_id + workspace_id; anything else is rejected rather than written.
func validateIssueInviteBuiltinScope(adminID int, wsID, typeKey string, submission *issueInviteSubmission) error {
	for key, table := range issueInviteBuiltinScopeTables {
		value, set := submission.Builtins[key]
		if !set {
			continue
		}
		id, _ := value.(int)
		if id <= 0 {
			return ErrIssueInviteFieldInvalid
		}
		query := [][]string{
			{`admin_id`, cast.ToString(adminID)},
			{`workspace_id`, wsID},
			{`id`, cast.ToString(id)},
		}
		if key == `priority` || key == `status` {
			query = append(query, []string{`type_key`, typeKey})
		}
		m := msql.Model(table, define.Postgres)
		count, err := m.Where2(query).Count()
		if err != nil {
			return err
		}
		if count <= 0 {
			return ErrIssueInviteFieldInvalid
		}
	}

	ids, assigneeSet := submission.Builtins[`assignee`].([]int)
	if assigneeSet && len(ids) > 0 {
		m := msql.Model(define.TableWorkspaceMembers, define.Postgres)
		rows, err := m.Where2([][]string{
			{`admin_id`, cast.ToString(adminID)},
			{`workspace_id`, wsID},
		}).Field(`user_id`).Select()
		if err != nil {
			return err
		}
		members := make(map[int]struct{}, len(rows))
		for _, row := range rows {
			members[cast.ToInt(row[`user_id`])] = struct{}{}
		}
		seen := make(map[int]struct{}, len(ids))
		for _, userID := range ids {
			if userID <= 0 {
				return ErrIssueInviteFieldInvalid
			}
			if _, ok := members[userID]; !ok {
				return ErrIssueInviteFieldInvalid
			}
			if _, dup := seen[userID]; dup {
				return ErrIssueInviteFieldInvalid
			}
			seen[userID] = struct{}{}
		}
	}
	return nil
}

// prepareIssueInviteSubmission normalizes one submitted form against the template and re-checks
// everything the public endpoint must not trust: custom values against the current project field
// configs, and the submitter-chosen builtin ids against the resolved project scope. Only the
// template-declared fields are required here, so hidden project-required fields that the template
// does not expose never block the external submitter. It returns the submittable builtin columns
// so the caller can map the chosen values onto the requirement columns.
func prepareIssueInviteSubmission(scope define.IssueInviteConfigModel, template define.IssueInviteTemplateModel, fields []define.IssueInviteFieldValue) (issueInviteSubmission, map[string]string, error) {
	submission, err := buildIssueInviteSubmission(template, fields)
	if err != nil {
		return issueInviteSubmission{}, nil, err
	}
	if err = validate.ValidateRequirementCreate(submission.Title, template.TypeKey, 0); err != nil {
		return issueInviteSubmission{}, nil, err
	}
	wsID := cast.ToString(scope.WorkspaceID)
	if err = ValidateCustomFieldValues(scope.AdminID, wsID, template.TypeKey, submission.CustomValues, false); err != nil {
		// 原样抛出：这个错误带着字段名和具体原因（数字/日期/选项不合法…），压成通用提示用户就不知道是哪个字段
		logs.Error("validate issue invite custom values failed, err:%s", err.Error())
		return issueInviteSubmission{}, nil, err
	}
	// The public form is unauthenticated, so an id from another tenant must never be written.
	if err = validateIssueInviteBuiltinScope(scope.AdminID, wsID, template.TypeKey, &submission); err != nil {
		logs.Error("validate issue invite builtin scope failed, err:%s", err.Error())
		return issueInviteSubmission{}, nil, ErrIssueInviteFieldInvalid
	}
	writable, err := loadIssueInviteWritableBuiltins(scope.AdminID, wsID, template.TypeKey)
	if err != nil {
		return issueInviteSubmission{}, nil, err
	}
	return submission, writable, nil
}

// applyIssueInviteBuiltinColumns copies the submitted builtin values onto the requirement columns.
func applyIssueInviteBuiltinColumns(record map[string]any, builtins map[string]any, writable map[string]string) {
	for key, value := range builtins {
		column, known := writable[key]
		if !known {
			continue
		}
		if ids, isList := value.([]int); isList {
			record[column] = pq.Array(ids)
			continue
		}
		record[column] = value
	}
}

// SubmitPublicIssue atomically creates an issue and its public submitter/template association.
func SubmitPublicIssue(model define.IssueInviteSubmitModel) (define.IssueInviteRequirementModel, error) {
	if err := validate.IssueInviteSubmission(model); err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	// Rate-limit public submissions per submitter before any scope lookup.
	allowed, err := allowIssueInviteSubmitRate(model.OpenID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	if !allowed {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteRateLimited
	}
	scope, err := ResolvePublicIssueInvite(model.InviteKey)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}

	now := int64(tool.Time2Int())
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	slaEnabled, err := prepareSLAMutationTx(tx, scope.AdminID, scope.WorkspaceID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}

	// Lock the enabled invitation template inside the resolved project scope.
	templateRows, err := msql.RawValues(define.Postgres, `SELECT `+issueInviteTemplateFields+`, a.enabled, a.sort_order
		FROM `+define.TableWorkspaceIssueItemTemplates+` a
		JOIN `+define.TableWorkItemTemplates+` t
			ON t.admin_id = a.admin_id AND t.id = a.template_id
		WHERE a.admin_id = $1 AND a.workspace_id = $2 AND t.invite_key = $3
		FOR UPDATE OF a, t`, tx, scope.AdminID, scope.WorkspaceID, model.TemplateKey)
	if err != nil {
		logs.Error("lock issue invite template for submit failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}
	if len(templateRows) == 0 {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteTemplateNotFound
	}
	templateRow, err := toIssueInviteTemplateRow(templateRows[0])
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	if templateRow.Enabled != 1 {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteTemplateDisabled
	}
	template := rowToIssueInviteTemplateModel(templateRow, scope.AdminID, scope.WorkspaceID)

	submission, writable, err := prepareIssueInviteSubmission(scope, template, model.Fields)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}

	// Builtin values fall back to the project defaults; an invitation template carries no
	// defaults of its own, so only what the submitter filled overrides them.
	wsID := cast.ToString(scope.WorkspaceID)
	statusID, err := getInitialStatusId(scope.AdminID, wsID, template.TypeKey)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	// 提交人自己选了状态就按他选的（上面已校验过属于本项目本类型）
	if chosen, set := submission.Builtins[`status`].(int); set && chosen > 0 {
		statusID = chosen
	}
	descriptionMode := template.DescriptionMode
	if descriptionMode == `` {
		descriptionMode = define.DescriptionModeRichText
	}
	record := map[string]any{
		`admin_id`:           scope.AdminID,
		`workspace_id`:       scope.WorkspaceID,
		`parent_id`:          0,
		`title`:              submission.Title,
		`description`:        submission.Description,
		`description_mode`:   descriptionMode,
		`type`:               template.TypeKey,
		`status_id`:          statusID,
		`template_id`:        template.ID,
		`template_name`:      template.Name,
		`priority`:           getDefaultPriorityId(scope.AdminID, wsID, template.TypeKey),
		`version_id`:         0,
		`planned_start_date`: int64(0),
		`planned_end_date`:   int64(0),
		`assignee_ids`:       pq.Array([]int{}),
		`created_by`:         nil,
		`created_at`:         now,
		`updated_at`:         now,
	}
	applyIssueInviteBuiltinColumns(record, submission.Builtins, writable)
	// 列名来自上面的固定集合，排序只为让 SQL 稳定可读
	columns := make([]string, 0, len(record))
	for column := range record {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	placeholders := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns))
	for index, column := range columns {
		placeholders = append(placeholders, `$`+strconv.Itoa(index+1))
		args = append(args, record[column])
	}

	inserted, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableRequirements+` (`+
		strings.Join(columns, `, `)+`)
		VALUES (`+strings.Join(placeholders, `, `)+`)
		RETURNING id`, tx, args...)
	if err != nil {
		logs.Error("insert public issue requirement failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}
	if len(inserted) == 0 {
		return define.IssueInviteRequirementModel{}, errors.New("public issue insert returned no id")
	}
	requirementID := cast.ToInt(inserted[0][`id`])

	// Persist custom field values inside the transaction.
	if err = saveIssueInviteCustomFieldValuesTx(tx, scope.AdminID, requirementID, submission.CustomValues, now); err != nil {
		return define.IssueInviteRequirementModel{}, err
	}

	// Snapshot the template association for history rendering.
	if _, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceIssueRequirements+`
		(admin_id, workspace_id, openid, requirement_id, template_id, template_key_snapshot,
		 template_type_key, template_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`, tx,
		scope.AdminID, scope.WorkspaceID, model.OpenID, requirementID, template.ID, template.InviteKey,
		template.TypeKey, template.Name, now); err != nil {
		logs.Error("insert public issue requirement association failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}

	if err = createHistoryTx(tx, scope.AdminID, scope.WorkspaceID, requirementID, cast.ToInt(record[`created_by`]),
		define.HistoryActionCreated, ``, ``, submission.Title); err != nil {
		logs.Error("create public issue history failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}

	if slaEnabled && template.TypeKey == define.TypeKeyRequirement {
		if err = recordSLACreatedTx(tx, scope.AdminID, scope.WorkspaceID, requirementID, "public_submit"); err != nil {
			return define.IssueInviteRequirementModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit public issue submit failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}

	if len(model.FileIDs) > 0 {
		if err = BindPendingEnterpriseFiles(scope.AdminID, requirementID, template.TypeKey, submission.Title, model.FileIDs); err != nil {
			logs.Error("bind pending enterprise files after issue invite submit failed, err:%s", err.Error())
		} else if err = appendIssueInviteAttachments(scope.AdminID, scope.WorkspaceID, requirementID); err != nil {
			logs.Error("append attachments to public issue description failed, err:%s", err.Error())
		}
	}

	if err = RotateIssueInviteHistoryCache(scope.AdminID, scope.WorkspaceID, model.OpenID); err != nil {
		logs.Error("rotate issue invite history cache after submit failed, err:%s", err.Error())
	}

	return getPublicIssueDetailData(scope.AdminID, scope.WorkspaceID, model.OpenID, requirementID)
}

// imageAttachmentExts are the file extensions rendered inline as <img> in the appended
// attachment block; svg is intentionally excluded because inline SVG can carry scripts.
var imageAttachmentExts = map[string]bool{
	`jpg`: true, `jpeg`: true, `png`: true, `gif`: true, `webp`: true, `bmp`: true, `ico`: true,
}

// videoAttachmentExts are the file extensions rendered inline as <video> in the appended
// attachment block. Keep the set to browser-playable containers; anything else renders as
// a clickable link and stays openable outside the page.
var videoAttachmentExts = map[string]bool{
	`mp4`: true, `webm`: true, `ogg`: true, `mov`: true,
}

// issueInviteAttachmentBlockMarker 是附件块的开头，见 appendIssueInviteAttachments。
// 描述里第一次出现它的位置就是「用户正文」和「服务端追加的附件块」的分界。
const issueInviteAttachmentBlockMarker = `<br>附件：<br>`

// splitIssueInviteAttachmentBlock 把描述切成用户正文和服务端追加的附件块。
// 没有附件块时第二段为空串。
func splitIssueInviteAttachmentBlock(description string) (string, string) {
	index := strings.Index(description, issueInviteAttachmentBlockMarker)
	if index < 0 {
		return description, ``
	}
	return description[:index], description[index:]
}

// appendIssueInviteAttachments appends a plain-HTML attachment block to the requirement
// description: images render inline as <img>, videos as <video> with native controls, and
// other files become clickable links. The frontend sanitizes the whole description with
// DOMPurify before rendering, and file names/URLs are HTML-escaped here because names come
// from the submitter. The escaped file name also sits inside the <video> element as fallback
// text so plain-text summaries keep showing the file name.
func appendIssueInviteAttachments(adminID, workspaceID, requirementID int) error {
	files, err := ListAttachments(adminID, workspaceID, requirementID)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	var builder strings.Builder
	for i, file := range files {
		if i == 0 {
			builder.WriteString(`<br>附件：<br>`)
		} else {
			builder.WriteString(`<br>`)
		}
		name := html.EscapeString(cast.ToString(file[`file_name`]))
		url := html.EscapeString(cast.ToString(file[`stored_path`]))
		builder.WriteString(cast.ToString(i+1) + `. `)
		ext := strings.ToLower(cast.ToString(file[`file_ext`]))
		switch {
		case imageAttachmentExts[ext]:
			builder.WriteString(`<img src="` + url + `" alt="` + name + `">`)
		case videoAttachmentExts[ext]:
			builder.WriteString(`<video controls preload="metadata" src="` + url + `">` + name + `</video>`)
		default:
			builder.WriteString(`<a href="` + url + `">` + name + `</a>`)
		}
	}
	appendText := builder.String()
	_, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableRequirements+`
		SET description = description || $1, updated_at = $2
		WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`, nil,
		appendText, tool.Time2Int(), adminID, workspaceID, requirementID)
	if err != nil {
		logs.Error("append attachments to requirement description failed, err:%s", err.Error())
		return err
	}
	return nil
}

// saveIssueInviteCustomFieldValuesTx upserts the validated custom values inside the transaction.
func saveIssueInviteCustomFieldValuesTx(tx *sql.Tx, adminID, requirementID int, values map[string]any, now int64) error {
	if len(values) == 0 {
		return nil
	}
	for fieldID, value := range values {
		text, err := customFieldValueText(value)
		if err != nil {
			return err
		}
		if text == `` || text == `[]` {
			continue
		}
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableCustomFieldValues+`
			(admin_id, requirement_id, field_id, value_text, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
			ON CONFLICT (admin_id, requirement_id, field_id) DO UPDATE SET
				value_text = EXCLUDED.value_text, updated_at = EXCLUDED.updated_at`, tx,
			adminID, requirementID, cast.ToInt(fieldID), text, now); err != nil {
			logs.Error("save issue invite custom field value failed, err:%s", err.Error())
			return err
		}
	}
	return nil
}

// allowIssueInviteSubmitRate enforces the per-submitter per-minute submission limit.
// The key is hashed so the raw openid never appears in Redis.
func allowIssueInviteSubmitRate(openID string) (bool, error) {
	key := joinIssueInviteCacheKey(define.CacheKeyIssueInviteSubmitLimit, sha256OpenID(openID))
	ctx := context.Background()
	pipe := define.Redis.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Minute)
	if _, err := pipe.Exec(ctx); err != nil {
		logs.Error("issue invite submit rate check failed, err:%s", err.Error())
		return false, err
	}
	return incr.Val() <= define.IssueInviteSubmitRateLimit, nil
}

// ListPublicIssueHistory returns one cached ten-row page for the resolved submitter.
func ListPublicIssueHistory(inviteKey, openID string, page int) (define.IssueInviteHistoryModel, error) {
	page, err := validate.IssueInvitePage(page)
	if err != nil {
		return define.IssueInviteHistoryModel{}, err
	}
	if err = validate.IssueInviteOpenID(openID); err != nil {
		return define.IssueInviteHistoryModel{}, ErrIssueInviteInvalid
	}
	scope, err := ResolvePublicIssueInvite(inviteKey)
	if err != nil {
		return define.IssueInviteHistoryModel{}, err
	}
	return GetCachedIssueInviteHistory(scope, openID, page)
}

// GetPublicIssueDetail returns one cached read-only issue owned by the resolved submitter.
func GetPublicIssueDetail(inviteKey, openID string, requirementID int) (define.IssueInviteRequirementModel, error) {
	if requirementID <= 0 {
		return define.IssueInviteRequirementModel{}, ErrRequirementNotFound
	}
	if err := validate.IssueInviteOpenID(openID); err != nil {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteInvalid
	}
	scope, err := ResolvePublicIssueInvite(inviteKey)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	return GetCachedIssueInviteDetail(scope, openID, requirementID)
}

// issueInviteAssociation 是「某个 openid 的某张邀填工单」在关联表上的状态。
type issueInviteAssociation struct {
	TemplateID  int
	CreatedAt   int64
	RecalledAt  int64
	LastUrgedAt int64
}

// PublicIssueHandleRecordsModel 是邀填人处理记录的输入与结果。
type PublicIssueHandleRecordsModel struct {
	InviteKey, OpenID string
	RequirementId     int
	Records           []IssueInviteHandleRecordModel
}

// IssueInviteHandleRecordModel 是对邀填人公开的一条处理记录。
type IssueInviteHandleRecordModel struct {
	Type, Content, ActorName, FromName, ToName string
	ContentHTML                                string
	CreatedAt                                  int64
	Attachments                                []IssueInviteCommentAttachmentModel
}

// IssueInviteCommentAttachmentModel 只包含公开回复已绑定附件的展示信息。
type IssueInviteCommentAttachmentModel struct {
	Id              int
	Name, URL, Kind string
	Size            int64
}

// GetPublicIssueHandleRecords 合并明确公开的评论及附件与工单处理历史。
func GetPublicIssueHandleRecords(model PublicIssueHandleRecordsModel) (PublicIssueHandleRecordsModel, error) {
	inviteKey, openID, requirementID := model.InviteKey, model.OpenID, model.RequirementId
	if err := validate.IssueInviteOpenID(openID); err != nil {
		return PublicIssueHandleRecordsModel{}, ErrIssueInviteInvalid
	}
	scope, err := ResolvePublicIssueInvite(inviteKey)
	if err != nil {
		return PublicIssueHandleRecordsModel{}, err
	}
	association, err := loadPublicIssueAssociation(nil, scope, openID, requirementID)
	if err != nil {
		return PublicIssueHandleRecordsModel{}, err
	}

	comments, err := msql.RawValues(define.Postgres, `SELECT id, content, created_at FROM `+define.TableRequirementComments+`
		WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = $3 AND source = $4 AND is_public = TRUE
		ORDER BY id DESC`, nil, scope.AdminID, scope.WorkspaceID, requirementID, define.IssueInviteCommentSource)
	if err != nil {
		logs.Error("list public issue handle comments failed, err:%s", err.Error())
		return PublicIssueHandleRecordsModel{}, err
	}
	// 回复关系只用于展示类型，不再决定公开范围。
	submitterCommentIDs := make([]int, 0, len(comments))
	for _, row := range comments {
		submitterCommentIDs = append(submitterCommentIDs, cast.ToInt(row[`id`]))
	}
	replies, err := msql.RawValues(define.Postgres, `SELECT c.id, c.content, c.parent_id, c.at_invite, c.created_at,
  COALESCE(NULLIF(u.name, ''), u.username, c.author_name_snapshot, '') AS actor_name
  FROM `+define.TableRequirementComments+` c
  LEFT JOIN users u ON u.id = c.user_id AND u.admin_id = c.admin_id
  WHERE c.admin_id = $1 AND c.workspace_id = $2 AND c.requirement_id = $3
   AND c.source <> $4 AND c.is_public = TRUE
  ORDER BY c.id DESC`, nil, scope.AdminID, scope.WorkspaceID, requirementID, define.IssueInviteCommentSource)
	if err != nil {
		logs.Error("list public issue handle replies failed, err:%s", err.Error())
		return PublicIssueHandleRecordsModel{}, err
	}
	// 必须从公开评论反查已绑定文件；不能公开同工单的内部附件、未绑定上传或其它租户文件。
	files, err := msql.RawValues(define.Postgres, `SELECT f.id, f.comment_id, f.file_name, f.file_ext, f.file_size, f.stored_path
  FROM `+define.GetEnterpriseFileTable(scope.AdminID)+` f
  JOIN `+define.TableRequirementComments+` c
   ON c.admin_id = f.admin_id AND c.workspace_id = f.workspace_id
   AND c.requirement_id = f.source_id AND c.id = f.comment_id
  WHERE c.admin_id = $1 AND c.workspace_id = $2 AND c.requirement_id = $3
   AND c.is_public = TRUE
  ORDER BY f.id`, nil, scope.AdminID, scope.WorkspaceID, requirementID)
	if err != nil {
		logs.Error("list public comment attachments failed, err:%s", err.Error())
		return PublicIssueHandleRecordsModel{}, err
	}
	attachments := make(map[int][]IssueInviteCommentAttachmentModel)
	for _, file := range files {
		kind := `file`
		ext := strings.ToLower(file[`file_ext`])
		if imageAttachmentExts[ext] {
			kind = `image`
		} else if videoAttachmentExts[ext] {
			kind = `video`
		}
		commentID := cast.ToInt(file[`comment_id`])
		attachments[commentID] = append(attachments[commentID], IssueInviteCommentAttachmentModel{
			Id: cast.ToInt(file[`id`]), Name: file[`file_name`], URL: file[`stored_path`], Size: cast.ToInt64(file[`file_size`]), Kind: kind,
		})
	}
	histories, err := msql.RawValues(define.Postgres, `SELECT action, old_value, new_value, created_at FROM `+define.TableRequirementHistories+`
		WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = $3
			AND action IN ($4, $5, $6)
		ORDER BY id DESC`, nil, scope.AdminID, scope.WorkspaceID, requirementID,
		define.HistoryActionCreated, define.HistoryActionStatusChanged, define.HistoryActionIssueRecall)
	if err != nil {
		logs.Error("list public issue handle histories failed, err:%s", err.Error())
		return PublicIssueHandleRecordsModel{}, err
	}

	records := make([]IssueInviteHandleRecordModel, 0, len(comments)+len(replies)+len(histories)+2)
	for _, row := range comments {
		records = append(records, IssueInviteHandleRecordModel{
			Type:        define.IssueInviteRecordTypeComment,
			Content:     row[`content`],
			CreatedAt:   cast.ToInt64(row[`created_at`]),
			Attachments: attachments[cast.ToInt(row[`id`])],
		})
	}
	submitterCommentSet := make(map[int]bool, len(submitterCommentIDs))
	for _, commentID := range submitterCommentIDs {
		submitterCommentSet[commentID] = true
	}
	for _, row := range replies {
		// 回复邀填人比单纯@邀填人更具体，同时命中时按回复展示。
		recordType := define.IssueInviteRecordTypeReply
		if cast.ToInt(row[`at_invite`]) == 1 && !submitterCommentSet[cast.ToInt(row[`parent_id`])] {
			recordType = define.IssueInviteRecordTypeAt
		}
		records = append(records, IssueInviteHandleRecordModel{
			Type:        recordType,
			Content:     issueInvitePlainText(row[`content`]),
			ContentHTML: row[`content`],
			ActorName:   row[`actor_name`],
			CreatedAt:   cast.ToInt64(row[`created_at`]),
			Attachments: attachments[cast.ToInt(row[`id`])],
		})
	}
	hasCreated := false
	hasRecallHistory := false
	for _, row := range histories {
		record := IssueInviteHandleRecordModel{CreatedAt: cast.ToInt64(row[`created_at`])}
		switch row[`action`] {
		case define.HistoryActionCreated:
			record.Type = define.IssueInviteRecordTypeCreate
			hasCreated = true
		case define.HistoryActionIssueRecall:
			// 撤回历史行是持久的：重新提交清掉 recalled_at 后它还在，撤回记录照旧展示
			record.Type = define.IssueInviteRecordTypeRecall
			hasRecallHistory = true
		case define.HistoryActionStatusChanged:
			record.Type = define.IssueInviteRecordTypeStatusChange
			record.FromName = row[`old_value`]
			record.ToName = row[`new_value`]
		default:
			continue
		}
		records = append(records, record)
	}
	if !hasCreated {
		// 旧工单没有持久化创建历史时，按关联行的创建时间补展示记录。
		records = append(records, IssueInviteHandleRecordModel{
			Type: define.IssueInviteRecordTypeCreate, CreatedAt: association.CreatedAt,
		})
	}
	if association.RecalledAt > 0 && !hasRecallHistory {
		// 老数据兜底：历史行机制上线前的撤回只在关联行上留了时间，按它补一条；
		// 新撤回总有历史行，重新提交后也不会丢记录。
		records = append(records, IssueInviteHandleRecordModel{
			Type: define.IssueInviteRecordTypeRecall, CreatedAt: association.RecalledAt,
		})
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].CreatedAt != records[j].CreatedAt {
			return records[i].CreatedAt > records[j].CreatedAt
		}
		// A comment posted at the same second sits above the status change.
		return isIssueInviteCommentRecord(records[i].Type) && !isIssueInviteCommentRecord(records[j].Type)
	})
	model.Records = records
	return model, nil
}

// isIssueInviteCommentRecord reports whether the record type carries comment text.
func isIssueInviteCommentRecord(recordType string) bool {
	switch recordType {
	case define.IssueInviteRecordTypeComment, define.IssueInviteRecordTypeReply, define.IssueInviteRecordTypeAt:
		return true
	default:
		return false
	}
}

// CreatePublicIssueComment posts one submitter comment on the public issue detail page.
func CreatePublicIssueComment(inviteKey, openID string, requirementID int, content string) (int, error) {
	if err := validate.IssueInviteOpenID(openID); err != nil {
		return 0, ErrIssueInviteInvalid
	}
	// Reuse the per-submitter public submission rate limit to keep the anonymous feed clean.
	allowed, err := allowIssueInviteSubmitRate(openID)
	if err != nil {
		return 0, err
	}
	if !allowed {
		return 0, ErrIssueInviteRateLimited
	}
	scope, err := ResolvePublicIssueInvite(inviteKey)
	if err != nil {
		return 0, err
	}
	if _, err = loadPublicIssueAssociation(nil, scope, openID, requirementID); err != nil {
		return 0, err
	}

	now := int64(tool.Time2Int())
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableRequirementComments+`
		(admin_id, workspace_id, requirement_id, user_id, content, source, is_public, created_at, updated_at)
		VALUES ($1, $2, $3, 0, $4, $5, TRUE, $6, $6) RETURNING id`, tx,
		scope.AdminID, scope.WorkspaceID, requirementID, content, define.IssueInviteCommentSource, now)
	if err != nil {
		logs.Error("insert public issue comment failed, err:%s", err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 0, errors.New("public issue comment insert returned no id")
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableRequirements+` SET comment_count = comment_count + 1,
		updated_at = $1 WHERE admin_id = $2 AND workspace_id = $3 AND id = $4`, tx,
		now, scope.AdminID, scope.WorkspaceID, requirementID); err != nil {
		logs.Error("update public issue comment count failed, err:%s", err.Error())
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit public issue comment failed, err:%s", err.Error())
		return 0, err
	}
	return cast.ToInt(rows[0][`id`]), nil
}

// issueInviteRequirementState 是撤回/催单/重提要看的工单现状。
type issueInviteRequirementState struct {
	Type        string
	StatusID    int
	StatusName  string
	Final       bool
	AssigneeIDs []int
	// Description 带着服务端追加的附件块，重提时要把这块原样接回去
	Description string
}

// loadPublicIssueAssociation 确认这张工单是当前 openid 提交的，并取出关联行上的时间；
// 传了事务就锁住这一行，撤回/催单/重提要改它，避免同一张工单被并发改。
func loadPublicIssueAssociation(tx *sql.Tx, scope define.IssueInviteConfigModel, openID string, requirementID int) (issueInviteAssociation, error) {
	query := `SELECT template_id, created_at, recalled_at, last_urged_at
		FROM ` + define.TableWorkspaceIssueRequirements + `
		WHERE admin_id = $1 AND workspace_id = $2 AND openid = $3 AND requirement_id = $4`
	if tx != nil {
		query += ` FOR UPDATE`
	}
	rows, err := msql.RawValues(define.Postgres, query, tx, scope.AdminID, scope.WorkspaceID, openID, requirementID)
	if err != nil {
		logs.Error("load public issue association failed, err:%s", err.Error())
		return issueInviteAssociation{}, err
	}
	if len(rows) == 0 {
		return issueInviteAssociation{}, ErrRequirementNotFound
	}
	return issueInviteAssociation{
		TemplateID:  cast.ToInt(rows[0][`template_id`]),
		CreatedAt:   cast.ToInt64(rows[0][`created_at`]),
		RecalledAt:  cast.ToInt64(rows[0][`recalled_at`]),
		LastUrgedAt: cast.ToInt64(rows[0][`last_urged_at`]),
	}, nil
}

// lockPublicIssueRequirementTx 锁住工单并读出它当前的类型、状态和处理人。
func lockPublicIssueRequirementTx(tx *sql.Tx, scope define.IssueInviteConfigModel, requirementID int) (issueInviteRequirementState, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT r.type, r.status_id, r.assignee_ids, r.description,
		COALESCE(s.name, '') AS status_name
		FROM `+define.TableRequirements+` r
		LEFT JOIN `+define.TableWorkItemStatuses+` s
			ON s.admin_id = r.admin_id AND s.workspace_id = r.workspace_id AND s.type_key = r.type AND s.id = r.status_id
		WHERE r.admin_id = $1 AND r.workspace_id = $2 AND r.id = $3
		FOR UPDATE OF r`, tx, scope.AdminID, scope.WorkspaceID, requirementID)
	if err != nil {
		logs.Error("lock public issue requirement failed, err:%s", err.Error())
		return issueInviteRequirementState{}, err
	}
	if len(rows) == 0 {
		return issueInviteRequirementState{}, ErrRequirementNotFound
	}
	state := issueInviteRequirementState{
		Type:        rows[0][`type`],
		StatusID:    cast.ToInt(rows[0][`status_id`]),
		StatusName:  rows[0][`status_name`],
		AssigneeIDs: parseStoredAssigneeIDs(rows[0][`assignee_ids`]),
		Description: rows[0][`description`],
	}
	final, err := isFinalIssueInviteStatus(scope.AdminID, scope.WorkspaceID, state.Type, state.StatusID)
	if err != nil {
		return issueInviteRequirementState{}, err
	}
	state.Final = final
	return state, nil
}

// isFinalIssueInviteStatus 判断工单是不是已经到终态；终态就是项目设置里该类型配的结束状态。
func isFinalIssueInviteStatus(adminID, workspaceID int, typeKey string, statusID int) (bool, error) {
	if statusID <= 0 {
		return false, nil
	}
	_, finalField := settingsStatusFields(typeKey)
	m := msql.Model(define.TableWorkspaceSettings, define.Postgres)
	raw, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`workspace_id`, cast.ToString(workspaceID)},
	}).Value(finalField)
	if err != nil {
		logs.Error("get issue invite final statuses failed, sql:%s, err:%s", m.GetLastSql(), err.Error())
		return false, err
	}
	for _, id := range parsePositiveInts(raw) {
		if id == statusID {
			return true, nil
		}
	}
	return false, nil
}

// applyIssueInviteUrgeWindow 按当前时间算能不能催单。已撤回或已结束的工单（CanModify 为真）不再催单。
func applyIssueInviteUrgeWindow(detail *define.IssueInviteRequirementModel, now int64) {
	detail.CanUrge = false
	detail.UrgeAvailableAt = 0
	if detail.CanModify {
		return
	}
	// 第一次催单不设门槛；催过之后两次之间才隔一个间隔。
	// 窗口不再跟提交时间挂钩——撤回重提的工单 created_at 还是首次提交那一刻，拿它算会把没催过的人也拦住。
	if detail.LastUrgedAt > 0 {
		availableAt := detail.LastUrgedAt + define.IssueInviteUrgeIntervalSeconds
		if now < availableAt {
			detail.UrgeAvailableAt = availableAt
			return
		}
	}
	detail.CanUrge = true
}

// loadIssueInviteStatusNameTx 读状态名，顺带确认这个状态还在这个项目里。
func loadIssueInviteStatusNameTx(tx *sql.Tx, scope define.IssueInviteConfigModel, typeKey string, statusID int) (string, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT name FROM `+define.TableWorkItemStatuses+`
		WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND id = $4`, tx,
		scope.AdminID, scope.WorkspaceID, typeKey, statusID)
	if err != nil {
		logs.Error("load issue invite status name failed, err:%s", err.Error())
		return ``, err
	}
	if len(rows) == 0 {
		return ``, ErrIssueInviteRecallStatusInvalid
	}
	return rows[0][`name`], nil
}

// lockIssueInviteTemplateTx 按模板 id 锁住模板行，重新提交时用它取字段 schema。
func lockIssueInviteTemplateTx(tx *sql.Tx, scope define.IssueInviteConfigModel, templateID int) (define.IssueInviteTemplateModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT `+issueInviteTemplateFields+`
		FROM `+define.TableWorkspaceIssueItemTemplates+` a
		JOIN `+define.TableWorkItemTemplates+` t
			ON t.admin_id = a.admin_id AND t.id = a.template_id
		WHERE a.admin_id = $1 AND a.workspace_id = $2 AND a.template_id = $3
		FOR UPDATE OF a, t`, tx, scope.AdminID, scope.WorkspaceID, templateID)
	if err != nil {
		logs.Error("lock issue invite template for resubmit failed, err:%s", err.Error())
		return define.IssueInviteTemplateModel{}, err
	}
	if len(rows) == 0 {
		return define.IssueInviteTemplateModel{}, ErrIssueInviteTemplateNotFound
	}
	converted, err := toIssueInviteTemplateRow(rows[0])
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	return rowToIssueInviteTemplateModel(converted, scope.AdminID, scope.WorkspaceID), nil
}

// deleteIssueInviteCustomFieldValuesTx 删掉这张工单上这批字段的旧值，重新提交时整份覆盖用。
func deleteIssueInviteCustomFieldValuesTx(tx *sql.Tx, adminID, requirementID int, fieldIDs []int) error {
	if len(fieldIDs) == 0 {
		return nil
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableCustomFieldValues+`
		WHERE admin_id = $1 AND requirement_id = $2 AND field_id = ANY($3::INTEGER[])`, tx,
		adminID, requirementID, pq.Array(fieldIDs)); err != nil {
		logs.Error("delete issue invite custom field values failed, err:%s", err.Error())
		return err
	}
	return nil
}

// refreshPublicIssueCaches 是撤回/催单/重提提交后统一要做的缓存刷新。
func refreshPublicIssueCaches(scope define.IssueInviteConfigModel, openID string, requirementID int) {
	if err := InvalidateIssueInviteDetailCache(scope.AdminID, scope.WorkspaceID, openID, requirementID); err != nil {
		logs.Error("invalidate issue invite detail cache failed, err:%s", err.Error())
	}
	if err := RotateIssueInviteHistoryCache(scope.AdminID, scope.WorkspaceID, openID); err != nil {
		logs.Error("rotate issue invite history cache failed, err:%s", err.Error())
	}
}

// RecallPublicIssue 让外部提交人撤回自己提交的工单：状态落到模板配的撤回状态，并记一条状态变更历史。
// 已经结束的工单不走撤回，用户要改就用重新提交。
func RecallPublicIssue(model define.IssueInviteRequirementActionModel) (define.IssueInviteRequirementModel, error) {
	if err := validate.IssueInviteOpenID(model.OpenID); err != nil {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteInvalid
	}
	scope, err := ResolvePublicIssueInvite(model.InviteKey)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	now := int64(tool.Time2Int())
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	slaEnabled, err := prepareSLAMutationTx(tx, scope.AdminID, scope.WorkspaceID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}

	association, err := loadPublicIssueAssociation(tx, scope, model.OpenID, model.RequirementID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	if association.RecalledAt > 0 {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteRecallNotAllowed
	}
	recallStatusID, err := loadIssueInviteRecallStatusTx(tx, scope, association.TemplateID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	requirement, err := lockPublicIssueRequirementTx(tx, scope, model.RequirementID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	var slaBefore SLAWorkItemStateModel
	if slaEnabled {
		slaBefore, err = lockSLAWorkItemTx(tx, scope.AdminID, scope.WorkspaceID, model.RequirementID)
		if err != nil {
			return define.IssueInviteRequirementModel{}, err
		}
	}
	if requirement.Final {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteRecallNotAllowed
	}

	// 撤回历史行是撤回的持久痕迹：重新提交会清掉关联行的 recalled_at，但这条历史还在，
	// 处理记录里的「撤回了工单」不丢。落到哪个状态由 new_value 带出。
	newName, nameErr := loadIssueInviteStatusNameTx(tx, scope, requirement.Type, recallStatusID)
	if nameErr != nil {
		return define.IssueInviteRequirementModel{}, nameErr
	}
	if recallStatusID != requirement.StatusID {
		if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableRequirements+`
			SET status_id = $1, updated_at = $2
			WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`, tx,
			recallStatusID, now, scope.AdminID, scope.WorkspaceID, model.RequirementID); err != nil {
			logs.Error("update recalled issue status failed, err:%s", err.Error())
			return define.IssueInviteRequirementModel{}, err
		}
	}
	if err = createHistoryTx(tx, scope.AdminID, scope.WorkspaceID, model.RequirementID, 0,
		define.HistoryActionIssueRecall, `status`, requirement.StatusName, newName); err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceIssueRequirements+`
		SET recalled_at = $1, updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND openid = $4 AND requirement_id = $5`, tx,
		now, scope.AdminID, scope.WorkspaceID, model.OpenID, model.RequirementID); err != nil {
		logs.Error("mark public issue recalled failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}
	if slaEnabled && recallStatusID != requirement.StatusID {
		if err = recordSLAUpdatedTx(tx, scope.AdminID, scope.WorkspaceID, model.RequirementID, slaBefore, "public_recall"); err != nil {
			return define.IssueInviteRequirementModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit public issue recall failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}

	refreshPublicIssueCaches(scope, model.OpenID, model.RequirementID)
	return getPublicIssueDetailData(scope.AdminID, scope.WorkspaceID, model.OpenID, model.RequirementID)
}

// loadIssueInviteRecallStatusTx 读模板上的撤回配置，返回撤回后要落的状态 id。
func loadIssueInviteRecallStatusTx(tx *sql.Tx, scope define.IssueInviteConfigModel, templateID int) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT recall_enabled, recall_status_id
		FROM `+define.TableWorkspaceIssueItemTemplates+`
		WHERE admin_id = $1 AND workspace_id = $2 AND template_id = $3`, tx,
		scope.AdminID, scope.WorkspaceID, templateID)
	if err != nil {
		logs.Error("load issue invite recall config failed, err:%s", err.Error())
		return 0, err
	}
	if len(rows) == 0 || cast.ToInt(rows[0][`recall_enabled`]) != 1 {
		return 0, ErrIssueInviteRecallDisabled
	}
	statusID := cast.ToInt(rows[0][`recall_status_id`])
	if statusID <= 0 {
		return 0, ErrIssueInviteRecallStatusInvalid
	}
	return statusID, nil
}

// UrgePublicIssue 让外部提交人催单：记一次催单时间，并把通知发给这张工单的处理人。
func UrgePublicIssue(model define.IssueInviteRequirementActionModel) (define.IssueInviteRequirementModel, error) {
	if err := validate.IssueInviteOpenID(model.OpenID); err != nil {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteInvalid
	}
	scope, err := ResolvePublicIssueInvite(model.InviteKey)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	now := int64(tool.Time2Int())
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	association, err := loadPublicIssueAssociation(tx, scope, model.OpenID, model.RequirementID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	requirement, err := lockPublicIssueRequirementTx(tx, scope, model.RequirementID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	if association.RecalledAt > 0 || requirement.Final {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteUrgeNotAllowed
	}
	// 跟详情接口的窗口口径一致：没催过直接催，催过才要隔一个间隔
	if association.LastUrgedAt > 0 {
		if availableAt := association.LastUrgedAt + define.IssueInviteUrgeIntervalSeconds; now < availableAt {
			return define.IssueInviteRequirementModel{}, ErrIssueInviteUrgeTooSoon
		}
	}

	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceIssueRequirements+`
		SET last_urged_at = $1, updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND openid = $4 AND requirement_id = $5`, tx,
		now, scope.AdminID, scope.WorkspaceID, model.OpenID, model.RequirementID); err != nil {
		logs.Error("mark public issue urged failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}
	// 催单文案对每个处理人都一样，先算一次；外部提交人没有站内账号，actor 留空。
	event := notificationEvent{
		AdminID:     scope.AdminID,
		WorkspaceID: scope.WorkspaceID,
		ItemID:      model.RequirementID,
		EventType:   define.NotificationEventIssueInviteUrged,
	}
	event.Content = notificationContent(event, ``, ``)
	for _, assigneeID := range requirement.AssigneeIDs {
		event.RecipientID = assigneeID
		if err = CreateNotificationTx(tx, event); err != nil {
			logs.Error("create issue invite urge notification failed, err:%s", err.Error())
			return define.IssueInviteRequirementModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit public issue urge failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}

	refreshPublicIssueCaches(scope, model.OpenID, model.RequirementID)
	return getPublicIssueDetailData(scope.AdminID, scope.WorkspaceID, model.OpenID, model.RequirementID)
}

// ResubmitPublicIssue 让外部提交人改完再重新提交：字段按模板重新校验并整份覆盖，状态回到初始状态，
// 撤回标记清掉。已结束和已撤回的工单都能走这条路。
func ResubmitPublicIssue(model define.IssueInviteRequirementActionModel) (define.IssueInviteRequirementModel, error) {
	if err := validate.IssueInviteOpenID(model.OpenID); err != nil {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteInvalid
	}
	scope, err := ResolvePublicIssueInvite(model.InviteKey)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	now := int64(tool.Time2Int())
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	slaEnabled, err := prepareSLAMutationTx(tx, scope.AdminID, scope.WorkspaceID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}

	association, err := loadPublicIssueAssociation(tx, scope, model.OpenID, model.RequirementID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	requirement, err := lockPublicIssueRequirementTx(tx, scope, model.RequirementID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	var slaBefore SLAWorkItemStateModel
	if slaEnabled {
		slaBefore, err = lockSLAWorkItemTx(tx, scope.AdminID, scope.WorkspaceID, model.RequirementID)
		if err != nil {
			return define.IssueInviteRequirementModel{}, err
		}
	}
	if association.RecalledAt == 0 && !requirement.Final {
		return define.IssueInviteRequirementModel{}, ErrIssueInviteResubmitNotAllowed
	}
	template, err := lockIssueInviteTemplateTx(tx, scope, association.TemplateID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	submission, writable, err := prepareIssueInviteSubmission(scope, template, model.Fields)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	wsID := cast.ToString(scope.WorkspaceID)
	statusID, err := getInitialStatusId(scope.AdminID, wsID, template.TypeKey)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	// 提交人自己选了状态就按他选的，下面的状态变更历史也按它比对
	if chosen, set := submission.Builtins[`status`].(int); set && chosen > 0 {
		statusID = chosen
	}
	descriptionMode := template.DescriptionMode
	if descriptionMode == `` {
		descriptionMode = define.DescriptionModeRichText
	}
	// 附件块是服务端拼进描述的，用户编辑的只是正文。没传新附件就把原来的块接回去，
	// 传了新附件则交给下面的 appendIssueInviteAttachments 按全部附件重列一份
	description := submission.Description
	if len(model.FileIDs) == 0 {
		_, block := splitIssueInviteAttachmentBlock(requirement.Description)
		description += block
	}
	record := map[string]any{
		`title`:            submission.Title,
		`description`:      description,
		`description_mode`: descriptionMode,
		`status_id`:        statusID,
		`updated_at`:       now,
	}
	applyIssueInviteBuiltinColumns(record, submission.Builtins, writable)
	// 列名来自上面的固定集合，排序只为让 SQL 稳定可读
	columns := make([]string, 0, len(record))
	for column := range record {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	assignments := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)+3)
	for _, column := range columns {
		args = append(args, record[column])
		assignments = append(assignments, column+` = $`+strconv.Itoa(len(args)))
	}
	args = append(args, scope.AdminID, scope.WorkspaceID, model.RequirementID)
	where := ` WHERE admin_id = $` + strconv.Itoa(len(args)-2) +
		` AND workspace_id = $` + strconv.Itoa(len(args)-1) + ` AND id = $` + strconv.Itoa(len(args))
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableRequirements+`
		SET `+strings.Join(assignments, `, `)+where, tx, args...); err != nil {
		logs.Error("update resubmitted public issue failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}
	// 模板里显示的字段整份覆盖：先删旧值，用户清空的字段才不会留着上一次的。
	if err = deleteIssueInviteCustomFieldValuesTx(tx, scope.AdminID, model.RequirementID, submission.DeclaredCustom); err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	if err = saveIssueInviteCustomFieldValuesTx(tx, scope.AdminID, model.RequirementID, submission.CustomValues, now); err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceIssueRequirements+`
		SET recalled_at = 0, updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND openid = $4 AND requirement_id = $5`, tx,
		now, scope.AdminID, scope.WorkspaceID, model.OpenID, model.RequirementID); err != nil {
		logs.Error("clear public issue recalled flag failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}
	if statusID != requirement.StatusID {
		newName, nameErr := loadIssueInviteStatusNameTx(tx, scope, requirement.Type, statusID)
		if nameErr != nil {
			return define.IssueInviteRequirementModel{}, nameErr
		}
		if err = createHistoryTx(tx, scope.AdminID, scope.WorkspaceID, model.RequirementID, 0,
			define.HistoryActionStatusChanged, `status`, requirement.StatusName, newName); err != nil {
			return define.IssueInviteRequirementModel{}, err
		}
	}
	if _, changed := record["assignee_ids"]; changed {
		after, err := getRequirementById(tx, scope.AdminID, wsID, cast.ToString(model.RequirementID))
		if err != nil {
			return define.IssueInviteRequirementModel{}, err
		}
		oldValue := assigneeIDsText(requirement.AssigneeIDs)
		newValue := assigneeIDsText(parseStoredAssigneeIDs(after["assignee_ids"]))
		if oldValue != newValue {
			if err := createHistoryTx(tx, scope.AdminID, scope.WorkspaceID, model.RequirementID, 0, define.HistoryActionAssignChanged, "assignee", oldValue, newValue); err != nil {
				return define.IssueInviteRequirementModel{}, err
			}
		}
	}
	if slaEnabled {
		if err = recordSLAResubmittedTx(tx, scope.AdminID, scope.WorkspaceID, model.RequirementID, slaBefore); err != nil {
			return define.IssueInviteRequirementModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit public issue resubmit failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}

	if len(model.FileIDs) > 0 {
		if err = BindPendingEnterpriseFiles(scope.AdminID, model.RequirementID, template.TypeKey, submission.Title, model.FileIDs); err != nil {
			logs.Error("bind pending enterprise files after issue invite resubmit failed, err:%s", err.Error())
		} else if err = appendIssueInviteAttachments(scope.AdminID, scope.WorkspaceID, model.RequirementID); err != nil {
			logs.Error("append attachments to resubmitted issue description failed, err:%s", err.Error())
		}
	}
	refreshPublicIssueCaches(scope, model.OpenID, model.RequirementID)
	return getPublicIssueDetailData(scope.AdminID, scope.WorkspaceID, model.OpenID, model.RequirementID)
}
