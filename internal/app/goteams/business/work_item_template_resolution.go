// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"encoding/json"
	"fmt"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

type templateNamedIDs struct {
	ByID   map[int]string
	ByName map[string]int
}

type templateFieldContext struct {
	WorkspaceID    int
	Builtins       map[string]bool
	Statuses       templateNamedIDs
	Priorities     templateNamedIDs
	Versions       templateNamedIDs
	Members        map[int]bool
	Categories     templateNamedIDs
	CustomByID     map[int]CustomFieldModel
	CustomBySchema map[string]CustomFieldModel
}

func resolveWorkItemTemplates(adminId int, targetWorkspaceId, typeKey string, items []WorkItemTemplateModel, strict bool) ([]WorkItemTemplateModel, error) {
	target, err := loadTemplateFieldContext(adminId, targetWorkspaceId, typeKey)
	if err != nil {
		return nil, err
	}
	sourceContexts := make(map[int]templateFieldContext)
	result := make([]WorkItemTemplateModel, 0, len(items))
	for _, item := range items {
		sourceWorkspaceId := item.SourceWorkspaceId
		if sourceWorkspaceId <= 0 {
			sourceWorkspaceId = cast.ToInt(targetWorkspaceId)
		}
		source, ok := sourceContexts[sourceWorkspaceId]
		if !ok {
			source, err = loadTemplateFieldContext(adminId, cast.ToString(sourceWorkspaceId), typeKey)
			if err != nil {
				return nil, err
			}
			sourceContexts[sourceWorkspaceId] = source
		}
		fields, resolveErr := resolveTemplateFields(item.Fields, source, target, strict)
		if resolveErr != nil {
			return nil, resolveErr
		}
		item.Fields = fields
		item.FieldCount = len(fields)
		result = append(result, item)
	}
	return result, nil
}

func ensureTemplateCompatible(adminId int, targetWorkspaceId, typeKey string, templateId int) error {
	m := msql.Model(define.TableWorkItemTemplates, define.Postgres)
	row, err := m.Field(`id,source_workspace_id,type_key,name,title,description,description_mode,fields_json,created_by,created_at,updated_at`).Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(templateId)},
		{`type_key`, typeKey},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	if len(row) == 0 {
		return ErrWorkItemTemplateNotFound
	}
	item, err := toWorkItemTemplateModel(row)
	if err != nil {
		return err
	}
	_, err = resolveWorkItemTemplates(adminId, targetWorkspaceId, typeKey, []WorkItemTemplateModel{item}, true)
	return err
}

func loadTemplateFieldContext(adminId int, workspaceId, typeKey string) (templateFieldContext, error) {
	context := templateFieldContext{
		WorkspaceID:    cast.ToInt(workspaceId),
		Builtins:       make(map[string]bool),
		Members:        make(map[int]bool),
		CustomByID:     make(map[int]CustomFieldModel),
		CustomBySchema: make(map[string]CustomFieldModel),
	}
	configs, err := ListFieldConfigs(ListFieldConfigsModel{AdminId: adminId, WsId: workspaceId, TypeKey: typeKey})
	if err != nil {
		return templateFieldContext{}, err
	}
	// Templates may reference fields whose type-level enable switch is off
	// (the manage-fields dialog lists every field of the type); validity is
	// about existence, not the switch.
	for _, field := range configs.Items {
		context.Builtins[field.Key] = true
	}
	context.Statuses, err = loadTemplateNamedIDs(define.TableWorkItemStatuses, adminId, workspaceId, typeKey, true)
	if err != nil {
		return templateFieldContext{}, err
	}
	context.Priorities, err = loadTemplateNamedIDs(define.TableWorkItemPriorities, adminId, workspaceId, typeKey, true)
	if err != nil {
		return templateFieldContext{}, err
	}
	context.Versions, err = loadTemplateNamedIDs(define.TableVersions, adminId, workspaceId, typeKey, false)
	if err != nil {
		return templateFieldContext{}, err
	}
	members, err := ListMembersByWorkspace(ListWorkspaceMembersModel{AdminId: adminId, WorkspaceId: workspaceId})
	if err != nil {
		return templateFieldContext{}, err
	}
	for _, member := range members.Items {
		if member.UserStatus != 0 && !member.Removed {
			context.Members[member.UserId] = true
		}
	}
	if typeKey == define.TypeKeyRequirement {
		context.Categories, err = loadTemplateNamedIDs(define.TableRequirementCategories, adminId, workspaceId, typeKey, false)
		if err != nil {
			return templateFieldContext{}, err
		}
	}
	customs, err := ListCustomFields(ListCustomFieldsModel{AdminId: adminId, WsId: workspaceId, TypeKey: typeKey})
	if err != nil {
		return templateFieldContext{}, err
	}
	// Same as builtins: keep disabled custom fields resolvable for templates.
	for _, field := range customs.Items {
		context.CustomByID[field.Id] = field
		schema := templateCustomSchema(field.Name, field.FieldType)
		if _, exists := context.CustomBySchema[schema]; !exists {
			context.CustomBySchema[schema] = field
		}
	}
	return context, nil
}

func loadTemplateNamedIDs(table string, adminId int, workspaceId, typeKey string, scoped bool) (templateNamedIDs, error) {
	query := fmt.Sprintf(`SELECT id, name FROM %s WHERE admin_id = $1 AND workspace_id = $2`, table)
	args := []any{adminId, cast.ToInt(workspaceId)}
	if scoped {
		query += ` AND type_key = $3`
		args = append(args, typeKey)
	}
	rows, err := msql.RawValues(define.Postgres, query, nil, args...)
	if err != nil {
		logs.Error("load template field options failed, table:%s, err:%s", table, err.Error())
		return templateNamedIDs{}, err
	}
	result := templateNamedIDs{ByID: make(map[int]string), ByName: make(map[string]int)}
	for _, row := range rows {
		id, convErr := parseTemplateRowInt(row, `id`)
		if convErr != nil {
			return templateNamedIDs{}, convErr
		}
		name := row[`name`]
		result.ByID[id] = name
		if _, exists := result.ByName[name]; !exists {
			result.ByName[name] = id
		}
	}
	return result, nil
}

func resolveTemplateFields(fields []TemplateFieldModel, source, target templateFieldContext, strict bool) ([]TemplateFieldModel, error) {
	result := make([]TemplateFieldModel, 0, len(fields))
	keyMap := make(map[string]string, len(fields))
	for _, field := range fields {
		originalKey := field.FieldKey
		if customId := templateCustomFieldID(field.FieldKey); customId > 0 {
			sourceField, sourceExists := source.CustomByID[customId]
			fieldName, fieldType := field.FieldName, field.FieldType
			if fieldName == `` && sourceExists {
				fieldName, fieldType = sourceField.Name, sourceField.FieldType
			}
			targetField, exists := target.CustomBySchema[templateCustomSchema(fieldName, fieldType)]
			if source.WorkspaceID == target.WorkspaceID && sourceExists {
				targetField, exists = target.CustomByID[customId]
			}
			if !exists {
				if strict {
					return nil, ErrWorkItemTemplateFieldInvalid
				}
				continue
			}
			field.FieldKey = `custom:` + cast.ToString(targetField.Id)
			field.FieldName = targetField.Name
			field.FieldType = targetField.FieldType
			if source.WorkspaceID != target.WorkspaceID {
				if len(field.DefaultNames) > 0 {
					field.DefaultValue = mapCustomTemplateDefaultNames(field.DefaultNames, targetField)
				} else if sourceExists {
					field.DefaultValue = resolveCustomTemplateDefault(field.DefaultValue, sourceField, targetField)
				}
			}
			keyMap[originalKey] = field.FieldKey
			result = append(result, field)
			continue
		}
		if !target.Builtins[field.FieldKey] {
			if strict {
				return nil, ErrWorkItemTemplateFieldInvalid
			}
			continue
		}
		if source.WorkspaceID != target.WorkspaceID {
			if len(field.DefaultNames) > 0 {
				field.DefaultValue = mapBuiltinTemplateDefaultNames(field.FieldKey, field.DefaultNames, target)
			} else {
				field.DefaultValue = resolveBuiltinTemplateDefault(field.FieldKey, field.DefaultValue, source, target)
			}
		}
		// 同一工作空间也要过滤：已离开项目的人仍在 metadata 里供历史展示，原样回显会被当成默认处理人。
		if field.FieldKey == `assignee` {
			field.DefaultValue = filterTemplateAssigneeIDs(field.DefaultValue, target.Members)
		}
		keyMap[originalKey] = field.FieldKey
		result = append(result, field)
	}
	for fieldIndex := range result {
		resolved := make([]TemplateLinkageModel, 0, len(result[fieldIndex].Linkages))
		for _, linkage := range result[fieldIndex].Linkages {
			if len(linkage.ValueNames) > 0 && source.WorkspaceID != target.WorkspaceID {
				linkage.Value = mapTemplateLinkageValue(result[fieldIndex], linkage.ValueNames, target)
			}
			if string(linkage.Value) == `null` {
				if strict {
					return nil, ErrWorkItemTemplateFieldInvalid
				}
				continue
			}
			fieldKeys := make([]string, 0, len(linkage.FieldKeys))
			for _, sourceKey := range linkage.FieldKeys {
				if targetKey, exists := keyMap[sourceKey]; exists {
					fieldKeys = append(fieldKeys, targetKey)
				} else if strict {
					return nil, ErrWorkItemTemplateFieldInvalid
				}
			}
			if len(fieldKeys) == 0 {
				continue
			}
			linkage.FieldKeys = fieldKeys
			resolved = append(resolved, linkage)
		}
		result[fieldIndex].Linkages = resolved
	}
	return result, nil
}

func mapTemplateLinkageValue(field TemplateFieldModel, names []string, target templateFieldContext) json.RawMessage {
	if customId := templateCustomFieldID(field.FieldKey); customId > 0 {
		targetField, exists := target.CustomByID[customId]
		if !exists || len(names) == 0 {
			return json.RawMessage(`null`)
		}
		key := templateOptionKey(targetField.Options, names[0])
		if key == `` {
			return json.RawMessage(`null`)
		}
		encoded, _ := json.Marshal(key)
		return encoded
	}
	return mapBuiltinTemplateDefaultNames(field.FieldKey, names, target)
}

func builtinTemplateDefaultNames(fieldKey string, raw json.RawMessage, context templateFieldContext) []string {
	var id int
	if len(raw) == 0 || json.Unmarshal(raw, &id) != nil || id <= 0 {
		return nil
	}
	var name string
	switch fieldKey {
	case `status`:
		name = context.Statuses.ByID[id]
	case `priority`:
		name = context.Priorities.ByID[id]
	case `version`:
		name = context.Versions.ByID[id]
	case `category`:
		name = context.Categories.ByID[id]
	}
	if name == `` {
		return nil
	}
	return []string{name}
}

func customTemplateDefaultNames(raw json.RawMessage, field CustomFieldModel) []string {
	if field.FieldType == `select_single` {
		var key string
		if json.Unmarshal(raw, &key) != nil {
			return nil
		}
		if name := templateOptionName(field.Options, key); name != `` {
			return []string{name}
		}
		return nil
	}
	if field.FieldType == `select_multi` {
		var keys []string
		if json.Unmarshal(raw, &keys) != nil {
			return nil
		}
		names := make([]string, 0, len(keys))
		for _, key := range keys {
			if name := templateOptionName(field.Options, key); name != `` {
				names = append(names, name)
			}
		}
		return names
	}
	return nil
}

func mapBuiltinTemplateDefaultNames(fieldKey string, names []string, target templateFieldContext) json.RawMessage {
	if len(names) == 0 {
		return json.RawMessage(`null`)
	}
	var id int
	switch fieldKey {
	case `status`:
		id = target.Statuses.ByName[names[0]]
	case `priority`:
		id = target.Priorities.ByName[names[0]]
	case `version`:
		id = target.Versions.ByName[names[0]]
	case `category`:
		id = target.Categories.ByName[names[0]]
	}
	if id <= 0 {
		return json.RawMessage(`null`)
	}
	encoded, _ := json.Marshal(id)
	return encoded
}

func mapCustomTemplateDefaultNames(names []string, target CustomFieldModel) json.RawMessage {
	if target.FieldType == `select_single` {
		if len(names) == 0 {
			return json.RawMessage(`null`)
		}
		key := templateOptionKey(target.Options, names[0])
		if key == `` {
			return json.RawMessage(`null`)
		}
		encoded, _ := json.Marshal(key)
		return encoded
	}
	if target.FieldType == `select_multi` {
		keys := make([]string, 0, len(names))
		for _, name := range names {
			if key := templateOptionKey(target.Options, name); key != `` {
				keys = append(keys, key)
			}
		}
		encoded, _ := json.Marshal(keys)
		return encoded
	}
	return json.RawMessage(`null`)
}

// filterTemplateAssigneeIDs 只保留当前项目成员，按原顺序去重。
// 已离开项目的 id 直接丢掉，避免模板默认处理人把他们画出来。
func filterTemplateAssigneeIDs(raw json.RawMessage, members map[int]bool) json.RawMessage {
	if templateDefaultIsEmpty(raw) {
		return raw
	}
	var ids []int
	if json.Unmarshal(raw, &ids) != nil {
		return raw
	}
	resolved := make([]int, 0, len(ids))
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || !members[id] || seen[id] {
			continue
		}
		seen[id] = true
		resolved = append(resolved, id)
	}
	encoded, err := json.Marshal(resolved)
	if err != nil {
		return raw
	}
	return encoded
}

func resolveBuiltinTemplateDefault(fieldKey string, raw json.RawMessage, source, target templateFieldContext) json.RawMessage {
	if source.WorkspaceID == target.WorkspaceID {
		return raw
	}
	switch fieldKey {
	case `status`:
		return mapTemplateNamedID(raw, source.Statuses, target.Statuses)
	case `priority`:
		return mapTemplateNamedID(raw, source.Priorities, target.Priorities)
	case `version`:
		return mapTemplateNamedID(raw, source.Versions, target.Versions)
	case `assignee`:
		return filterTemplateAssigneeIDs(raw, target.Members)
	case `category`:
		return mapTemplateNamedID(raw, source.Categories, target.Categories)
	default:
		return raw
	}
}

func mapTemplateNamedID(raw json.RawMessage, source, target templateNamedIDs) json.RawMessage {
	var id int
	if len(raw) == 0 || json.Unmarshal(raw, &id) != nil || id <= 0 {
		return raw
	}
	name, exists := source.ByID[id]
	if !exists {
		return json.RawMessage(`null`)
	}
	targetId, exists := target.ByName[name]
	if !exists {
		return json.RawMessage(`null`)
	}
	encoded, _ := json.Marshal(targetId)
	return encoded
}

func resolveCustomTemplateDefault(raw json.RawMessage, source, target CustomFieldModel) json.RawMessage {
	if source.FieldType == `select_single` {
		var key string
		if json.Unmarshal(raw, &key) != nil || key == `` {
			return raw
		}
		name := templateOptionName(source.Options, key)
		targetKey := templateOptionKey(target.Options, name)
		if targetKey == `` {
			return json.RawMessage(`null`)
		}
		encoded, _ := json.Marshal(targetKey)
		return encoded
	}
	if source.FieldType == `select_multi` {
		var keys []string
		if json.Unmarshal(raw, &keys) != nil {
			return raw
		}
		targetKeys := make([]string, 0, len(keys))
		for _, key := range keys {
			name := templateOptionName(source.Options, key)
			if targetKey := templateOptionKey(target.Options, name); targetKey != `` {
				targetKeys = append(targetKeys, targetKey)
			}
		}
		encoded, _ := json.Marshal(targetKeys)
		return encoded
	}
	return raw
}

func templateCustomFieldID(key string) int {
	const prefix = `custom:`
	if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
		return 0
	}
	return cast.ToInt(key[len(prefix):])
}

func templateCustomSchema(name, fieldType string) string {
	return fieldType + "\x00" + name
}

func templateOptionName(options []CustomFieldOption, key string) string {
	for _, option := range options {
		if option.Key == key {
			return option.Name
		}
	}
	return ``
}

func templateOptionKey(options []CustomFieldOption, name string) string {
	if name == `` {
		return ``
	}
	for _, option := range options {
		if option.Name == name {
			return option.Key
		}
	}
	return ``
}
