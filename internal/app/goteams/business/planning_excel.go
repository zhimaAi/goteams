// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/xuri/excelize/v2"
	"github.com/zhimaAi/go_tools/msql"
)

// planningExcelRelatedKey is the read-only "related requirements" column; export only, never imported.
const planningExcelRelatedKey = `related_items`

// BuildPlanningWorkbookModel describes the planning template / export workbook use case.
type BuildPlanningWorkbookModel struct {
	AdminId      int
	WorkspaceId  string
	GroupId      int
	Lang         string
	Timezone     string
	TemplateOnly bool
	// Filters, shared with the planning list so an export matches what is on screen.
	Title       string
	PriorityIds []int
	StatusIds   []int
	AssigneeId  int
	HasRelation *bool
	Archived    *int
}

// ImportPlanningsModel describes the planning import use case; PlanningIds is unused, GroupId decides where new rows land.
type ImportPlanningsModel struct {
	AdminId     int
	UserId      int
	WorkspaceId string
	GroupId     int
	Lang        string
	Timezone    string
	DryRun      bool
}

// BuildPlanningWorkbook generates the planning import template or export workbook.
func BuildPlanningWorkbook(model BuildPlanningWorkbookModel) ([]byte, error) {
	columns, err := buildPlanningExcelColumns(model.AdminId, model.WorkspaceId, model.Lang, !model.TemplateOnly)
	if err != nil {
		return nil, err
	}
	location, err := loadWorkItemLocation(model.Timezone)
	if err != nil {
		return nil, err
	}

	file := excelize.NewFile()
	sheetName := lib_i18n.Show(model.Lang, `planning_excel_sheet`)
	if err = file.SetSheetName(`Sheet1`, sheetName); err != nil {
		return nil, err
	}
	// Reuse the work-item header/meta writers: the layout and the column-mapping sheet are identical.
	if err = writeWorkItemWorkbookHeader(file, sheetName, columns, model.TemplateOnly); err != nil {
		return nil, err
	}
	if err = writeWorkItemWorkbookMeta(file, columns, define.TypeKeyPlanning); err != nil {
		return nil, err
	}
	if !model.TemplateOnly {
		listModel, listErr := ListPlannings(ListPlanningsModel{
			AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, GroupId: model.GroupId, Lang: model.Lang,
			Title: model.Title, PriorityIds: model.PriorityIds, StatusIds: model.StatusIds,
			AssigneeId: model.AssigneeId, HasRelation: model.HasRelation, Archived: model.Archived, Size: workItemMaxRows,
		})
		if listErr != nil {
			return nil, listErr
		}
		if err = writePlanningWorkbookRows(file, sheetName, columns, listModel.Items, model, location); err != nil {
			return nil, err
		}
	}
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// buildPlanningExcelColumns assembles the planning Excel columns following the project's field configuration.
// The "related requirements" column only exists on export, because relations are not importable.
func buildPlanningExcelColumns(adminId int, wsId, lang string, includeAuditColumns bool) ([]WorkItemExcelColumn, error) {
	inputs, err := loadWorkItemExcelInputs(adminId, wsId, define.TypeKeyPlanning, lang)
	if err != nil {
		return nil, err
	}
	return buildPlanningExcelColumnsFrom(inputs, lang, includeAuditColumns), nil
}

// buildPlanningExcelColumnsFrom assembles the planning columns from already-loaded configuration.
// Pure function: no database access, so the column layout invariants are unit testable.
func buildPlanningExcelColumnsFrom(inputs workItemExcelInputs, lang string, includeAuditColumns bool) []WorkItemExcelColumn {
	configMap := make(map[string]FieldConfigModel, len(inputs.configs))
	for _, item := range inputs.configs {
		configMap[item.Key] = item
	}
	label := func(key, fallback string) string {
		if item, ok := configMap[key]; ok && item.Label != `` {
			return item.Label
		}
		return lib_i18n.Show(lang, fallback)
	}
	// Display-field gate only; see the note on the work-item version. ID never goes through it.
	enabled := func(key string) bool {
		item, ok := configMap[key]
		return !ok || item.Enabled == 1
	}
	description := func(key string) string {
		if values := inputs.options[key]; len(values) > 0 {
			return strings.Join(values, `/`)
		}
		return lib_i18n.Show(lang, `workitem_excel_desc_`+key)
	}

	columns := make([]WorkItemExcelColumn, 0, 12)
	appendBuiltin := func(key, fallback string, readOnly bool) {
		if enabled(key) {
			columns = append(columns, WorkItemExcelColumn{Key: key, Label: label(key, fallback), Description: description(key), ReadOnly: readOnly})
		}
	}
	columns = append(columns, newWorkItemExcelIDColumn(label(workItemExcelIDKey, `workspace_field_id`), description(workItemExcelIDKey)))
	appendBuiltin(`title`, `planning_excel_title`, false)
	appendBuiltin(`priority`, `workspace_field_priority`, false)
	if includeAuditColumns && enabled(planningExcelRelatedKey) {
		columns = append(columns, WorkItemExcelColumn{
			Key:         planningExcelRelatedKey,
			Label:       label(planningExcelRelatedKey, `planning_excel_related_items`),
			Description: lib_i18n.Show(lang, `planning_excel_desc_related_items`),
			ReadOnly:    true,
		})
	}
	appendBuiltin(`status`, `workspace_field_status`, false)
	appendBuiltin(`assignee`, `workspace_field_assignee`, false)
	appendBuiltin(`planned_start_date`, `workspace_field_planned_start_date`, false)
	appendBuiltin(`planned_end_date`, `workspace_field_planned_end_date`, false)
	if includeAuditColumns {
		appendBuiltin(`created_at`, `workspace_field_created_time`, true)
		appendBuiltin(`created_by`, `workspace_field_created_by`, true)
	}

	customFields := inputs.customFields
	for i := range customFields {
		field := customFields[i]
		if field.IsEnabled != 1 {
			continue
		}
		desc := lib_i18n.Show(lang, `workitem_excel_desc_custom`)
		if field.FieldType == `select_single` || field.FieldType == `select_multi` {
			names := make([]string, 0, len(field.Options))
			for _, option := range field.Options {
				names = append(names, option.Name)
			}
			desc = fmt.Sprintf(lib_i18n.Show(lang, `workitem_excel_desc_custom_options`), strings.Join(names, `/`))
		}
		columns = append(columns, WorkItemExcelColumn{
			Key: `custom:` + cast.ToString(field.Id), Label: field.Name, Description: desc, CustomField: &field,
		})
	}
	return columns
}

// writePlanningWorkbookRows writes the planning rows; names come from the project's status/priority/member configuration.
func writePlanningWorkbookRows(file *excelize.File, sheetName string, columns []WorkItemExcelColumn,
	items []PlanningModel, model BuildPlanningWorkbookModel, location *time.Location) error {
	names, err := buildPlanningExportNames(model.AdminId, model.WorkspaceId, model.Lang)
	if err != nil {
		return err
	}
	for rowIndex, item := range items {
		customValues, valuesErr := ListCustomFieldValues(model.AdminId, item.Id)
		if valuesErr != nil {
			return valuesErr
		}
		for colIndex, column := range columns {
			cell, cellErr := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			if cellErr != nil {
				return cellErr
			}
			value := planningExportValue(column, item, customValues, names, model.Lang, location)
			if err = file.SetCellValue(sheetName, cell, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildPlanningExportNames caches the display names needed by an export.
func listPlanningExportMembers(adminId int, wsId string) (map[int]string, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT u.id, u.name, u.username FROM workspace_members wm
		JOIN users u ON u.id = wm.user_id AND u.admin_id = wm.admin_id
		WHERE wm.admin_id = $1 AND wm.workspace_id = $2
		UNION
		SELECT ru.user_id AS id, ru.name, '' AS username FROM removed_users ru
		WHERE ru.admin_id = $1`, nil, adminId, cast.ToInt(wsId))
	if err != nil {
		return nil, err
	}
	members := make(map[int]string, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row[`name`])
		if name == `` {
			name = strings.TrimSpace(row[`username`])
		}
		members[cast.ToInt(row[`id`])] = name
	}
	return members, nil
}

// planningExportNames caches the display names needed by an export.
type planningExportNames struct {
	statuses   map[int]string
	priorities map[int]string
	members    map[int]string
}

func buildPlanningExportNames(adminId int, wsId, lang string) (planningExportNames, error) {
	names := planningExportNames{statuses: map[int]string{}, priorities: map[int]string{}, members: map[int]string{}}
	statusesModel, err := ListStatuses(ListStatusesModel{AdminId: adminId, WsId: wsId, TypeKey: define.TypeKeyPlanning, Lang: lang})
	if err != nil {
		return names, err
	}
	for _, item := range statusesModel.Items {
		names.statuses[item.Id] = item.Name
	}
	prioritiesModel, err := ListPriorities(ListPrioritiesModel{AdminId: adminId, WsId: wsId, TypeKey: define.TypeKeyPlanning, Lang: lang})
	if err != nil {
		return names, err
	}
	for _, item := range prioritiesModel.Items {
		names.priorities[item.Id] = item.Label
	}
	members, err := listPlanningExportMembers(adminId, wsId)
	if err != nil {
		return names, err
	}
	names.members = members
	return names, nil
}

func planningExportValue(column WorkItemExcelColumn, item PlanningModel, customValues map[string]string,
	names planningExportNames, lang string, location *time.Location) any {
	switch column.Key {
	case `id`:
		return cast.ToString(item.Id)
	case `title`:
		return item.Title
	case `priority`:
		return names.priorities[item.Priority]
	case planningExcelRelatedKey:
		// Related work items are exported as a readable "#id title" list; the column is ignored on import.
		texts := make([]string, 0, len(item.Relations))
		for _, relation := range item.Relations {
			texts = append(texts, fmt.Sprintf(`#%d %s`, relation.Id, relation.Title))
		}
		return strings.Join(texts, "\n")
	case `status`:
		return names.statuses[item.StatusId]
	case `assignee`:
		texts := make([]string, 0, len(item.AssigneeIds))
		for _, id := range item.AssigneeIds {
			if name := names.members[id]; name != `` {
				texts = append(texts, name)
			}
		}
		return strings.Join(texts, `,`)
	case `planned_start_date`:
		return formatWorkItemDate(item.PlannedStartDate, location)
	case `planned_end_date`:
		return formatWorkItemDate(item.PlannedEndDate, location)
	case `created_at`:
		return formatWorkItemTime(item.CreatedAt, location)
	case `created_by`:
		return names.members[item.CreatedBy]
	}
	if column.CustomField == nil {
		return ``
	}
	// Custom field rendering is identical to work items; reuse it via a zero-value RequirementModel.
	return workItemExportValue(column, RequirementModel{}, customValues, lang, location)
}

// planningImportMaxRows caps the rows one planning import may carry.
// Deliberately lower than the work-item limit: a single planning group holding thousands of
// rows is not a real use case, and the cap also keeps the batch INSERT parameter budget comfortable.
const planningImportMaxRows = 2000

// ImportPlannings creates or updates planning rows from the template workbook.
// Rows to create are collected and written by a single batched insert at the end,
// so the round-trip count stays flat no matter how many rows the file has.
func ImportPlannings(reader io.Reader, model ImportPlanningsModel) (WorkItemImportResult, error) {
	result := WorkItemImportResult{Errors: []WorkItemImportError{}}
	location, err := loadWorkItemLocation(model.Timezone)
	if err != nil {
		return result, err
	}
	file, err := excelize.OpenReader(reader)
	if err != nil {
		return result, err
	}
	defer func() { _ = file.Close() }()

	columns, err := buildPlanningExcelColumns(model.AdminId, model.WorkspaceId, model.Lang, true)
	if err != nil {
		return result, err
	}
	columnMap, err := readWorkItemColumnMap(file, columns, define.TypeKeyPlanning)
	if err != nil {
		return result, err
	}
	sheetName := firstWorkItemSheet(file)
	if sheetName == `` {
		return result, fmt.Errorf("planning sheet not found")
	}
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return result, err
	}
	dataStartRow := findWorkItemHeaderRow(rows, columns) + 1
	if len(rows) <= dataStartRow {
		return result, nil
	}
	if len(rows)-dataStartRow > planningImportMaxRows {
		return result, fmt.Errorf("too many rows: %d", len(rows)-dataStartRow)
	}
	lookups, err := buildWorkItemImportLookups(model.AdminId, model.WorkspaceId, define.TypeKeyPlanning, model.Lang)
	if err != nil {
		return result, err
	}
	// Loaded once for the whole file; every row validates against this list in memory.
	customFieldsModel, err := ListCustomFields(ListCustomFieldsModel{
		AdminId: model.AdminId, WsId: model.WorkspaceId, TypeKey: define.TypeKeyPlanning,
	})
	if err != nil {
		return result, err
	}
	customFields := customFieldsModel.Items
	lookups.customFieldMemberScope, err = loadCustomFieldMemberScopeForFields(model.AdminId, model.WorkspaceId, customFields)
	if err != nil {
		return result, err
	}
	customByID := make(map[int]CustomFieldModel)
	for _, column := range columns {
		if column.CustomField != nil {
			customByID[column.CustomField.Id] = *column.CustomField
		}
	}

	// Creates are buffered and flushed as one batch below; updates cannot be merged and run inline.
	drafts := make([]PlanningDraftModel, 0, len(rows)-dataStartRow)
	draftRows := make([]int, 0, len(rows)-dataStartRow)
	for index, row := range rows[dataStartRow:] {
		if workItemRowEmpty(row) {
			continue
		}
		result.Total++
		rowNumber := index + dataStartRow + 1
		idText := strings.TrimSpace(workItemCell(row, findWorkItemColumn(columnMap, `id`)))
		draft, rowErr := importPlanningRow(row, columnMap, customByID, customFields, lookups, model, location)
		if rowErr == nil {
			if idText == `` {
				result.Created++
				if draft != nil {
					drafts = append(drafts, *draft)
					draftRows = append(draftRows, rowNumber)
				}
			} else {
				result.Updated++
			}
			continue
		}
		if rowErr.skip {
			result.Skipped++
		} else {
			result.Failed++
		}
		result.Errors = append(result.Errors, WorkItemImportError{Row: rowNumber, Id: idText, Reason: rowErr.message})
	}

	// Flush the buffered creates. The batch is atomic, so a failure invalidates every row it carried.
	if len(drafts) > 0 {
		if _, err := BatchCreatePlannings(BatchCreatePlanningsModel{
			AdminId:     model.AdminId,
			UserId:      model.UserId,
			WorkspaceId: model.WorkspaceId,
			GroupId:     model.GroupId,
			Drafts:      drafts,
		}); err != nil {
			reason := localizedWorkItemImportError(err, model.Lang)
			result.Created -= len(drafts)
			result.Failed += len(drafts)
			for _, rowNumber := range draftRows {
				result.Errors = append(result.Errors, WorkItemImportError{Row: rowNumber, Reason: reason})
			}
		}
	}
	return result, nil
}

// PreviewPlannings pre-parses the planning file and counts importable rows without writing to the database.
func PreviewPlannings(reader io.Reader, model ImportPlanningsModel) (WorkItemImportPreview, error) {
	model.DryRun = true
	full, err := ImportPlannings(reader, model)
	if err != nil {
		return WorkItemImportPreview{}, err
	}
	return WorkItemImportPreview{
		Total:      full.Total,
		Importable: full.Created + full.Updated,
		Skipped:    full.Skipped,
		Failed:     full.Failed,
	}, nil
}

// importPlanningRow parses one Excel row. An empty ID yields a draft for the caller's batch insert;
// a filled ID updates in place and returns a nil draft. Dry runs validate only and never return a draft.
func importPlanningRow(row []string, columns map[int]string, customByID map[int]CustomFieldModel,
	customFields []CustomFieldModel, lookups workItemImportLookups, model ImportPlanningsModel,
	location *time.Location) (*PlanningDraftModel, *workItemImportRowError) {
	value := func(key string) string { return strings.TrimSpace(workItemCell(row, findWorkItemColumn(columns, key))) }
	lang := model.Lang
	idText := value(`id`)
	statusId := workItemLookupValue(lookups.statuses, value(`status`))
	priority := workItemLookupValue(lookups.priorities, value(`priority`))
	assigneeIds := workItemLookupValues(lookups.members, value(`assignee`))
	plannedStart, startOK := parseWorkItemTime(value(`planned_start_date`), location)
	plannedEnd, endOK := parseWorkItemTime(value(`planned_end_date`), location)
	customValues := parseWorkItemCustomValues(row, columns, customByID, location)
	start, end := optionalWorkItemTime(plannedStart, startOK), optionalWorkItemTime(plannedEnd, endOK)
	if err := validate.ValidatePlannedDateRange(start, end); err != nil {
		return nil, &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
	}

	if idText == `` {
		title := value(`title`)
		if title == `` {
			return nil, &workItemImportRowError{message: lib_i18n.Show(lang, `planning_import_title_required`)}
		}
		if err := validateCustomFieldValuesWithScope(customFields, customValues, true, lookups.customFieldMemberScope); err != nil {
			return nil, &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
		}
		if model.DryRun {
			return nil, nil
		}
		return &PlanningDraftModel{
			Title:            title,
			Priority:         priority,
			StatusId:         statusId,
			AssigneeIds:      assigneeIds,
			PlannedStartDate: start,
			PlannedEndDate:   end,
			CustomFields:     customValues,
		}, nil
	}

	id, ok := parseWorkItemID(idText)
	if !ok || id <= 0 {
		return nil, &workItemImportRowError{message: lib_i18n.Show(lang, `workitem_import_id_invalid`), skip: true}
	}
	if model.DryRun {
		exist, err := GetPlanningById(model.AdminId, model.WorkspaceId, id)
		if err != nil {
			return nil, &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
		}
		if !exist {
			return nil, &workItemImportRowError{message: lib_i18n.Show(lang, `workitem_import_id_not_current_project`), skip: true}
		}
		if err := validateCustomFieldValuesWithScope(customFields, customValues, true, lookups.customFieldMemberScope); err != nil {
			return nil, &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
		}
		return nil, nil
	}

	save := SavePlanningModel{
		AdminId: model.AdminId, UserId: model.UserId, WorkspaceId: model.WorkspaceId,
		GroupId: model.GroupId, PlanningId: id, CustomFields: customValues,
	}
	if statusId > 0 {
		save.StatusId = &statusId
	}
	if priority > 0 {
		save.Priority = &priority
	}
	if len(assigneeIds) > 0 {
		save.AssigneeIds = &assigneeIds
	}
	if startOK {
		save.PlannedStartDate = &start
	}
	if endOK {
		save.PlannedEndDate = &end
	}
	if title := value(`title`); title != `` {
		save.Title = &title
	}
	if err := UpdatePlanning(save); err != nil {
		if errors.Is(err, ErrPlanningNotFound) {
			return nil, &workItemImportRowError{message: lib_i18n.Show(lang, `workitem_import_id_not_current_project`), skip: true}
		}
		return nil, &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
	}
	return nil, nil
}
