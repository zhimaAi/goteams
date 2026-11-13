// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/xuri/excelize/v2"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const (
	workItemMetaSheet = `_goteams_meta`
	workItemMaxRows   = 10000
)

// WorkItemExcelColumn describes a column in the work-item Excel.
type WorkItemExcelColumn struct {
	Key         string
	Label       string
	Description string
	CustomField *CustomFieldModel
	// ReadOnly marks a column that is exported for reading but must never be written back.
	// Enforced in readWorkItemColumnMap, which drops these columns from the import mapping,
	// so the rule lives in code rather than in "nobody wrote a branch for it".
	ReadOnly bool
}

// workItemExcelIDKey is the import matching key; see newWorkItemExcelIDColumn.
const workItemExcelIDKey = `id`

// newWorkItemExcelIDColumn builds the ID column shared by every work-item-family Excel
// (requirement, defect and planning).
//
// Deliberately NOT routed through the field configuration: ID is the key that decides
// create-vs-update on import, not a display field. If a display setting could remove it,
// findWorkItemColumn would return -1, every row would look like a create, and the import
// would silently duplicate the user's data without a single error.
// TestWorkItemExcelKeepsIDColumn* lock this invariant.
func newWorkItemExcelIDColumn(label, description string) WorkItemExcelColumn {
	return WorkItemExcelColumn{Key: workItemExcelIDKey, Label: label, Description: description}
}

// workItemExcelInputs bundles everything the column builder reads from the database,
// so that assembling the columns stays a pure function and can be unit tested.
type workItemExcelInputs struct {
	configs      []FieldConfigModel
	customFields []CustomFieldModel
	options      map[string][]string
}

// WorkItemImportError describes the reason a single row failed to import.
type WorkItemImportError struct {
	Row    int    `json:"row"`
	Id     string `json:"id,omitempty"`
	Reason string `json:"reason"`
}

// WorkItemImportResult returns the work-item import summary.
type WorkItemImportResult struct {
	Total   int                   `json:"total"`
	Created int                   `json:"created"`
	Updated int                   `json:"updated"`
	Skipped int                   `json:"skipped"`
	Failed  int                   `json:"failed"`
	Errors  []WorkItemImportError `json:"errors"`
}

// WorkItemImportPreview returns the pre-parsed result of the work-item file without writing to the database.
type WorkItemImportPreview struct {
	Total      int `json:"total"`
	Importable int `json:"importable"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

// BuildWorkItemWorkbook generates the template or export workbook for the current work-item type.
// model carries the tenant, workspace, work-item type, and filters needed for export; filters are ignored when only exporting a template.
func BuildWorkItemWorkbook(model ListRequirementsModel, lang, timezone string, ids []int, templateOnly bool) ([]byte, error) {
	model.Timezone = timezone
	adminId := model.AdminId
	wsId := cast.ToString(model.WorkspaceId)
	typeKey := model.Type
	columns, err := buildWorkItemExcelColumns(adminId, wsId, typeKey, lang, !templateOnly)
	if err != nil {
		return nil, err
	}
	location, err := loadWorkItemLocation(timezone)
	if err != nil {
		return nil, err
	}

	file := excelize.NewFile()
	sheetName := lib_i18n.Show(lang, `workitem_excel_sheet`)
	if err = file.SetSheetName(`Sheet1`, sheetName); err != nil {
		return nil, err
	}
	if err = writeWorkItemWorkbookHeader(file, sheetName, columns, templateOnly); err != nil {
		return nil, err
	}
	if err = writeWorkItemWorkbookMeta(file, columns, typeKey); err != nil {
		return nil, err
	}
	if !templateOnly {
		items, listErr := listWorkItemsForExport(model, ids)
		if listErr != nil {
			return nil, listErr
		}
		if err = writeWorkItemWorkbookRows(file, sheetName, columns, items, adminId, lang, location, 2); err != nil {
			return nil, err
		}
	}
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func buildWorkItemExcelColumns(adminId int, wsId, typeKey, lang string, includeAuditColumns bool) ([]WorkItemExcelColumn, error) {
	inputs, err := loadWorkItemExcelInputs(adminId, wsId, typeKey, lang)
	if err != nil {
		return nil, err
	}
	return buildWorkItemExcelColumnsFrom(inputs, typeKey, lang, includeAuditColumns), nil
}

// loadWorkItemExcelInputs reads the workspace configuration the column layout depends on.
func loadWorkItemExcelInputs(adminId int, wsId, typeKey, lang string) (workItemExcelInputs, error) {
	configsModel, err := ListFieldConfigs(ListFieldConfigsModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey, Lang: lang})
	if err != nil {
		return workItemExcelInputs{}, err
	}
	options, err := buildWorkItemExcelDescriptionOptions(adminId, wsId, typeKey, lang)
	if err != nil {
		return workItemExcelInputs{}, err
	}
	customFieldsModel, err := ListCustomFields(ListCustomFieldsModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey})
	if err != nil {
		return workItemExcelInputs{}, err
	}
	return workItemExcelInputs{
		configs:      configsModel.Items,
		customFields: customFieldsModel.Items,
		options:      options,
	}, nil
}

// buildWorkItemExcelColumnsFrom assembles the columns from already-loaded configuration.
// Pure function: no database access, so the column layout invariants are unit testable.
func buildWorkItemExcelColumnsFrom(inputs workItemExcelInputs, typeKey, lang string, includeAuditColumns bool) []WorkItemExcelColumn {
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
	// Display-field gate only. A missing config row means the field exists in code but was not
	// seeded into this workspace yet, so it is shown rather than silently lost. Structural columns
	// such as ID must not be routed through here - see newWorkItemExcelIDColumn.
	enabled := func(key string) bool {
		item, ok := configMap[key]
		return !ok || item.Enabled == 1
	}
	description := func(key string) string {
		return lib_i18n.Show(lang, `workitem_excel_desc_`+key)
	}
	optionDescription := func(key string) string {
		if values := inputs.options[key]; len(values) > 0 {
			return strings.Join(values, `/`)
		}
		return description(key)
	}

	columns := make([]WorkItemExcelColumn, 0, 16)
	appendBuiltin := func(key, fallback string, readOnly bool) {
		if enabled(key) {
			columns = append(columns, WorkItemExcelColumn{Key: key, Label: label(key, fallback), Description: description(key), ReadOnly: readOnly})
		}
	}
	columns = append(columns, newWorkItemExcelIDColumn(label(workItemExcelIDKey, `workspace_field_id`), description(workItemExcelIDKey)))
	appendBuiltin(`title`, `workspace_field_title`, false)
	descriptionLabelKey := `workitem_excel_requirement_description`
	if typeKey == define.TypeKeyDefect {
		descriptionLabelKey = `workitem_excel_defect_description`
	}
	columns = append(columns, WorkItemExcelColumn{
		Key: `description`, Label: lib_i18n.Show(lang, descriptionLabelKey), Description: description(`description`),
	})
	appendOptionBuiltin := func(key, fallback string) {
		if enabled(key) {
			columns = append(columns, WorkItemExcelColumn{Key: key, Label: label(key, fallback), Description: optionDescription(key)})
		}
	}
	appendOptionBuiltin(`status`, `workspace_field_status`)
	appendOptionBuiltin(`version`, `workspace_field_iteration`)
	appendOptionBuiltin(`priority`, `workspace_field_priority`)
	appendOptionBuiltin(`assignee`, `workspace_field_assignee`)
	appendBuiltin(`planned_start_date`, `workspace_field_planned_start_date`, false)
	appendBuiltin(`planned_end_date`, `workspace_field_planned_end_date`, false)
	if typeKey == define.TypeKeyRequirement {
		appendBuiltin(`estimated_work_hours`, `workspace_field_estimated_work_hours`, false)
		appendBuiltin(`work_progress`, `workspace_field_work_progress`, true)
		appendBuiltin(`completed_work_hours`, `workspace_field_completed_work_hours`, true)
		appendBuiltin(`remaining_work_hours`, `workspace_field_remaining_work_hours`, true)
		appendBuiltin(`overtime_work_hours`, `workspace_field_overtime_work_hours`, true)
	}
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

func buildWorkItemExcelDescriptionOptions(adminId int, wsId, typeKey, lang string) (map[string][]string, error) {
	options := map[string][]string{
		`type`: {
			lib_i18n.Show(lang, `workitem_type_requirement`),
			lib_i18n.Show(lang, `workitem_type_defect`),
		},
	}
	statusesModel, err := ListStatuses(ListStatusesModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey, Lang: lang})
	if err != nil {
		return nil, err
	}
	for _, item := range statusesModel.Items {
		options[`status`] = append(options[`status`], item.Name)
	}
	prioritiesModel, err := ListPriorities(ListPrioritiesModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey, Lang: lang})
	if err != nil {
		return nil, err
	}
	for _, item := range prioritiesModel.Items {
		options[`priority`] = append(options[`priority`], item.Label)
	}
	members, err := msql.RawValues(define.Postgres, `SELECT u.name, u.username FROM workspace_members wm
		JOIN users u ON u.id = wm.user_id AND u.admin_id = wm.admin_id AND COALESCE(u.status, 1) = 1
		WHERE wm.admin_id = $1 AND wm.workspace_id = $2 ORDER BY u.name ASC, u.id ASC`, nil, adminId, cast.ToInt(wsId))
	if err != nil {
		return nil, err
	}
	for _, item := range members {
		name := strings.TrimSpace(item[`name`])
		if name == `` {
			name = strings.TrimSpace(item[`username`])
		}
		if name != `` {
			options[`assignee`] = append(options[`assignee`], name)
		}
	}
	versions, err := msql.RawValues(define.Postgres, `SELECT name FROM versions
		WHERE admin_id = $1 AND workspace_id = $2 ORDER BY id ASC`, nil, adminId, cast.ToInt(wsId))
	if err != nil {
		return nil, err
	}
	for _, item := range versions {
		if name := strings.TrimSpace(item[`name`]); name != `` {
			options[`version`] = append(options[`version`], name)
		}
	}
	return options, nil
}

func writeWorkItemWorkbookHeader(file *excelize.File, sheetName string, columns []WorkItemExcelColumn, includeDescriptions bool) error {
	headerStyle, err := file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: `262626`},
		Fill:      excelize.Fill{Type: `pattern`, Color: []string{`F5F7FA`}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: `center`},
		Border:    []excelize.Border{{Type: `bottom`, Color: `D9D9D9`, Style: 1}},
	})
	if err != nil {
		return err
	}
	headerRow := 1
	var descriptionStyle int
	var endDescriptionCell string
	if includeDescriptions {
		headerRow = 2
		descriptionStyle, err = file.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Color: `3A4559`, Size: 11},
			Fill:      excelize.Fill{Type: `pattern`, Color: []string{`E9F1FE`}, Pattern: 1},
			Alignment: &excelize.Alignment{WrapText: true, Vertical: `top`},
			Border:    []excelize.Border{{Type: `bottom`, Color: `99BFFD`, Style: 1}},
		})
		if err != nil {
			return err
		}
		if endDescriptionCell, err = excelize.CoordinatesToCellName(len(columns), 1); err != nil {
			return err
		}
	}
	for index, column := range columns {
		if includeDescriptions {
			cell, cellErr := excelize.CoordinatesToCellName(index+1, 1)
			if cellErr != nil {
				return cellErr
			}
			if err = file.SetCellValue(sheetName, cell, column.Description); err != nil {
				return err
			}
		}
		cell, cellErr := excelize.CoordinatesToCellName(index+1, headerRow)
		if cellErr != nil {
			return cellErr
		}
		if err = file.SetCellValue(sheetName, cell, column.Label); err != nil {
			return err
		}
		colName, colErr := excelize.ColumnNumberToName(index + 1)
		if colErr != nil {
			return colErr
		}
		width := 16.0
		if column.Key == `title` {
			width = 32
		}
		if strings.HasPrefix(column.Key, `planned_`) || column.Key == `created_at` {
			width = 20
		}
		if err = file.SetColWidth(sheetName, colName, colName, width); err != nil {
			return err
		}
	}
	// In template mode, set date columns to text format so that dates typed by users
	// (e.g. 2026-01-01) are not auto-recognized and converted to formats like 2026/01/01, which would cause data loss on import.
	// Must run before the description/header row styles: excelize SetColStyle iterates all existing cells in the column
	// and overrides their styles, so applying it afterwards would overwrite the description row's background color and wrap style.
	if includeDescriptions {
		textFormat := `@`
		dateTextStyle, styleErr := file.NewStyle(&excelize.Style{CustomNumFmt: &textFormat})
		if styleErr != nil {
			return styleErr
		}
		for index, column := range columns {
			if !isWorkItemDateColumn(column) {
				continue
			}
			colName, colErr := excelize.ColumnNumberToName(index + 1)
			if colErr != nil {
				return colErr
			}
			if err = file.SetColStyle(sheetName, colName, dateTextStyle); err != nil {
				return err
			}
		}
	}
	// The description row style must be applied after all values are written, so that every column remark
	// (including planned start/end and custom fields) is uniformly shown with a light-blue background and word wrap.
	if includeDescriptions {
		if err = file.SetCellStyle(sheetName, `A1`, endDescriptionCell, descriptionStyle); err != nil {
			return err
		}
		if err = file.SetRowHeight(sheetName, 1, 58); err != nil {
			return err
		}
	}
	endCell, err := excelize.CoordinatesToCellName(len(columns), headerRow)
	if err != nil {
		return err
	}
	startCell, err := excelize.CoordinatesToCellName(1, headerRow)
	if err != nil {
		return err
	}
	if err = file.SetCellStyle(sheetName, startCell, endCell, headerStyle); err != nil {
		return err
	}
	if err = file.SetRowHeight(sheetName, headerRow, 24); err != nil {
		return err
	}
	dataStartCell, err := excelize.CoordinatesToCellName(1, headerRow+1)
	if err != nil {
		return err
	}
	if err = file.SetPanes(sheetName, &excelize.Panes{Freeze: true, YSplit: headerRow, TopLeftCell: dataStartCell, ActivePane: `bottomLeft`}); err != nil {
		return err
	}
	return file.AutoFilter(sheetName, startCell+`:`+endCell, []excelize.AutoFilterOptions{})
}

func writeWorkItemWorkbookMeta(file *excelize.File, columns []WorkItemExcelColumn, typeKey string) error {
	if _, err := file.NewSheet(workItemMetaSheet); err != nil {
		return err
	}
	headers := []string{`column`, `key`, `custom_field_id`, `field_type`, `type_key`}
	for index, header := range headers {
		cell, err := excelize.CoordinatesToCellName(index+1, 1)
		if err != nil {
			return err
		}
		if err = file.SetCellValue(workItemMetaSheet, cell, header); err != nil {
			return err
		}
	}
	for index, column := range columns {
		row := index + 2
		values := []any{index + 1, column.Key, 0, ``, typeKey}
		if column.CustomField != nil {
			values[2] = column.CustomField.Id
			values[3] = column.CustomField.FieldType
		}
		for col, value := range values {
			cell, err := excelize.CoordinatesToCellName(col+1, row)
			if err != nil {
				return err
			}
			if err = file.SetCellValue(workItemMetaSheet, cell, value); err != nil {
				return err
			}
		}
	}
	return file.SetSheetVisible(workItemMetaSheet, false)
}

// listWorkItemsForExport pulls exportable work items page by page per the filters; when ids is non-empty, only the specified rows are kept.
func listWorkItemsForExport(model ListRequirementsModel, ids []int) ([]RequirementModel, error) {
	allowed := make(map[int]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}
	items := make([]RequirementModel, 0)
	for page := 1; ; page++ {
		model.Page = page
		model.Size = 100
		result, err := ListRequirements(model)
		if err != nil {
			return nil, err
		}
		if len(result.Items) == 0 {
			break
		}
		for _, item := range result.Items {
			if len(allowed) == 0 || allowed[item.Id] {
				items = append(items, item)
			}
		}
		if page*100 >= result.Total {
			break
		}
	}
	return items, nil
}

func writeWorkItemWorkbookRows(file *excelize.File, sheetName string, columns []WorkItemExcelColumn, items []RequirementModel, adminId int, lang string, location *time.Location, startRow int) error {
	for rowIndex, item := range items {
		customValues, err := ListCustomFieldValues(adminId, item.Id)
		if err != nil {
			return err
		}
		for colIndex, column := range columns {
			value := workItemExportValue(column, item, customValues, lang, location)
			cell, cellErr := excelize.CoordinatesToCellName(colIndex+1, rowIndex+startRow)
			if cellErr != nil {
				return cellErr
			}
			if err = file.SetCellValue(sheetName, cell, value); err != nil {
				return err
			}
		}
	}
	endCell, err := excelize.CoordinatesToCellName(len(columns), len(items)+startRow-1)
	if err != nil {
		return err
	}
	return file.AutoFilter(sheetName, `A1:`+endCell, []excelize.AutoFilterOptions{})
}

func workItemExportValue(column WorkItemExcelColumn, item RequirementModel, customValues map[string]string, lang string, location *time.Location) any {
	switch column.Key {
	case `id`:
		return cast.ToString(item.Id)
	case `title`:
		return item.Title
	case `description`:
		return item.Description
	case `status`:
		return lib_i18n.ShowStored(lang, item.StatusName)
	case `version`:
		return item.VersionName
	case `priority`:
		return lib_i18n.ShowStored(lang, item.PriorityName)
	case `assignee`:
		return strings.Join(item.AssigneeNames, `,`)
	case `planned_start_date`:
		return formatWorkItemDate(item.PlannedStartDate, location)
	case `planned_end_date`:
		return formatWorkItemDate(item.PlannedEndDate, location)
	case `estimated_work_hours`:
		return item.EstimatedWorkHours
	case `work_progress`:
		return item.WorkProgress
	case `completed_work_hours`:
		return item.CompletedWorkHours
	case `remaining_work_hours`:
		return item.RemainingWorkHours
	case `overtime_work_hours`:
		return item.OvertimeWorkHours
	case `created_at`:
		return formatWorkItemTime(item.CreatedAt, location)
	case `created_by`:
		return item.CreatorName
	}
	if column.CustomField == nil {
		return ``
	}
	raw := customValues[cast.ToString(column.CustomField.Id)]
	if column.CustomField.FieldType == `select_single` {
		return customOptionName(*column.CustomField, raw)
	}
	if column.CustomField.FieldType == `select_multi` {
		keys := []string{}
		if err := tool.JsonDecode(raw, &keys); err != nil {
			return ``
		}
		names := make([]string, 0, len(keys))
		for _, key := range keys {
			if name := customOptionName(*column.CustomField, key); name != `` {
				names = append(names, name)
			}
		}
		return strings.Join(names, `,`)
	}
	if column.CustomField.FieldType == `date` {
		return formatWorkItemCustomDate(raw, location)
	}
	return raw
}

// formatWorkItemCustomDate outputs a custom date field value: if it is stored as a date string, output it as-is;
// it also tolerates timestamps (plain numbers) mistakenly stored by historical imports, formatting them as timestamps in that case.
func formatWorkItemCustomDate(raw string, location *time.Location) string {
	if _, err := time.Parse(`2006-01-02`, strings.TrimSpace(raw)); err == nil {
		return strings.TrimSpace(raw)
	}
	return formatWorkItemTime(cast.ToInt64(raw), location)
}

func customOptionName(field CustomFieldModel, key string) string {
	for _, option := range field.Options {
		if option.Key == key {
			return option.Name
		}
	}
	return ``
}

// ImportWorkItems creates or updates work items row by row from the template workbook.
func ImportWorkItems(reader io.Reader, adminId int, wsId, typeKey, lang, timezone string, userId int) (WorkItemImportResult, error) {
	result := WorkItemImportResult{Errors: []WorkItemImportError{}}
	location, err := loadWorkItemLocation(timezone)
	if err != nil {
		return result, err
	}
	file, err := excelize.OpenReader(reader)
	if err != nil {
		return result, err
	}
	defer func() { _ = file.Close() }()

	columns, err := buildWorkItemExcelColumns(adminId, wsId, typeKey, lang, true)
	if err != nil {
		return result, err
	}
	columnMap, err := readWorkItemColumnMap(file, columns, typeKey)
	if err != nil {
		return result, err
	}
	sheetName := firstWorkItemSheet(file)
	if sheetName == `` {
		return result, fmt.Errorf("work item sheet not found")
	}
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return result, err
	}
	headerRow := findWorkItemHeaderRow(rows, columns)
	dataStartRow := headerRow + 1
	if len(rows) <= dataStartRow {
		return result, nil
	}
	if len(rows)-dataStartRow > workItemMaxRows {
		return result, fmt.Errorf("too many rows: %d", len(rows)-dataStartRow)
	}
	lookups, err := buildWorkItemImportLookups(adminId, wsId, typeKey, lang)
	if err != nil {
		return result, err
	}
	customByID := make(map[int]CustomFieldModel)
	for _, column := range columns {
		if column.CustomField != nil {
			customByID[column.CustomField.Id] = *column.CustomField
		}
	}

	for index, row := range rows[dataStartRow:] {
		rowNumber := index + dataStartRow + 1
		if workItemRowEmpty(row) {
			continue
		}
		result.Total++
		idText := strings.TrimSpace(workItemCell(row, findWorkItemColumn(columnMap, `id`)))
		rowErr := importWorkItemRow(row, columnMap, customByID, lookups, adminId, wsId, typeKey, lang, location, userId, false)
		if rowErr == nil {
			if idText == `` {
				result.Created++
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
	return result, nil
}

// PreviewWorkItems pre-parses the work-item file and counts importable rows without performing create or update.
func PreviewWorkItems(reader io.Reader, adminId int, wsId, typeKey, lang, timezone string, userId int) (WorkItemImportPreview, error) {
	result := WorkItemImportPreview{}
	location, err := loadWorkItemLocation(timezone)
	if err != nil {
		return result, err
	}
	file, err := excelize.OpenReader(reader)
	if err != nil {
		return result, err
	}
	defer func() { _ = file.Close() }()
	columns, err := buildWorkItemExcelColumns(adminId, wsId, typeKey, lang, true)
	if err != nil {
		return result, err
	}
	columnMap, err := readWorkItemColumnMap(file, columns, typeKey)
	if err != nil {
		return result, err
	}
	sheetName := firstWorkItemSheet(file)
	if sheetName == `` {
		return result, fmt.Errorf("work item sheet not found")
	}
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return result, err
	}
	headerRow := findWorkItemHeaderRow(rows, columns)
	dataStartRow := headerRow + 1
	if len(rows) <= dataStartRow {
		return result, nil
	}
	if len(rows)-dataStartRow > workItemMaxRows {
		return result, fmt.Errorf("too many rows: %d", len(rows)-dataStartRow)
	}
	lookups, err := buildWorkItemImportLookups(adminId, wsId, typeKey, lang)
	if err != nil {
		return result, err
	}
	customByID := make(map[int]CustomFieldModel)
	for _, column := range columns {
		if column.CustomField != nil {
			customByID[column.CustomField.Id] = *column.CustomField
		}
	}
	for _, row := range rows[dataStartRow:] {
		if workItemRowEmpty(row) {
			continue
		}
		result.Total++
		rowErr := importWorkItemRow(row, columnMap, customByID, lookups, adminId, wsId, typeKey, lang, location, userId, true)
		if rowErr == nil {
			result.Importable++
		} else if rowErr.skip {
			result.Skipped++
		} else {
			result.Failed++
		}
	}
	return result, nil
}

type workItemImportRowError struct {
	message string
	skip    bool
}

type workItemImportLookups struct {
	statuses               map[string]int
	priorities             map[string]int
	members                map[string]int
	versions               map[string]int
	customFieldMemberScope *customFieldMemberScope
}

func buildWorkItemImportLookups(adminId int, wsId, typeKey, lang string) (workItemImportLookups, error) {
	lookups := workItemImportLookups{statuses: map[string]int{}, priorities: map[string]int{}, members: map[string]int{}, versions: map[string]int{}}
	statusesModel, err := ListStatuses(ListStatusesModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey, Lang: lang})
	if err != nil {
		return lookups, err
	}
	for _, item := range statusesModel.Items {
		lookups.statuses[normalizeWorkItemLookup(item.Name)] = item.Id
		lookups.statuses[cast.ToString(item.Id)] = item.Id
	}
	prioritiesModel, err := ListPriorities(ListPrioritiesModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey, Lang: lang})
	if err != nil {
		return lookups, err
	}
	for _, item := range prioritiesModel.Items {
		lookups.priorities[normalizeWorkItemLookup(item.Label)] = item.Id
		lookups.priorities[cast.ToString(item.Id)] = item.Id
	}
	members, err := msql.RawValues(define.Postgres, `SELECT u.id, u.name, u.username FROM workspace_members wm
		JOIN users u ON u.id = wm.user_id AND u.admin_id = wm.admin_id
		WHERE wm.admin_id = $1 AND wm.workspace_id = $2`, nil, adminId, cast.ToInt(wsId))
	if err != nil {
		return lookups, err
	}
	for _, item := range members {
		lookups.members[normalizeWorkItemLookup(item[`name`])] = cast.ToInt(item[`id`])
		lookups.members[normalizeWorkItemLookup(item[`username`])] = cast.ToInt(item[`id`])
		lookups.members[item[`id`]] = cast.ToInt(item[`id`])
	}
	versions, err := msql.RawValues(define.Postgres, `SELECT id, name FROM versions WHERE admin_id = $1 AND workspace_id = $2`, nil, adminId, cast.ToInt(wsId))
	if err != nil {
		return lookups, err
	}
	for _, item := range versions {
		lookups.versions[normalizeWorkItemLookup(item[`name`])] = cast.ToInt(item[`id`])
		lookups.versions[item[`id`]] = cast.ToInt(item[`id`])
	}
	return lookups, nil
}

func importWorkItemRow(row []string, columns map[int]string, customByID map[int]CustomFieldModel, lookups workItemImportLookups, adminId int, wsId, typeKey, lang string, location *time.Location, userId int, dryRun bool) *workItemImportRowError {
	value := func(key string) string { return strings.TrimSpace(workItemCell(row, findWorkItemColumn(columns, key))) }
	idText := value(`id`)
	// The import template no longer contains a type column; the import type is taken uniformly from the request parameter type
	// (importing a requirement list means requirement, importing a defect list means defect).
	itemType := typeKey

	statusID := workItemLookupValue(lookups.statuses, value(`status`))
	priorityID := workItemLookupValue(lookups.priorities, value(`priority`))
	assigneeIDs := workItemLookupValues(lookups.members, value(`assignee`))
	versionID := workItemLookupValue(lookups.versions, value(`version`))
	plannedStart, startOK := parseWorkItemTime(value(`planned_start_date`), location)
	plannedEnd, endOK := parseWorkItemTime(value(`planned_end_date`), location)
	estimatedWorkHours := 0.0
	if rawHours := value(`estimated_work_hours`); rawHours != `` {
		parsedHours, parseErr := strconv.ParseFloat(rawHours, 64)
		if parseErr != nil || validate.ValidateRequirementWorkHours(parsedHours) != nil {
			return &workItemImportRowError{message: lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid)}
		}
		estimatedWorkHours = parsedHours
	}
	customValues := parseWorkItemCustomValues(row, columns, customByID, location)

	if idText == `` {
		if err := validate.ValidatePlannedDateRange(optionalWorkItemTime(plannedStart, startOK), optionalWorkItemTime(plannedEnd, endOK)); err != nil {
			return &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
		}
		title := value(`title`)
		if title == `` {
			return &workItemImportRowError{message: lib_i18n.Show(lang, `workitem_import_title_required`)}
		}
		if dryRun {
			if err := ValidateCustomFieldValues(adminId, wsId, itemType, customValues, true); err != nil {
				return &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
			}
			return nil
		}
		var priority *int
		if priorityID > 0 {
			priority = &priorityID
		}
		if _, err := CreateRequirement(CreateRequirementModel{
			AdminId:            adminId,
			UserId:             userId,
			WorkspaceId:        cast.ToInt(wsId),
			Title:              title,
			Description:        value(`description`),
			Type:               itemType,
			Priority:           priority,
			StatusId:           statusID,
			AssigneeIds:        assigneeIDs,
			VersionId:          versionID,
			PlannedStartDate:   optionalWorkItemTime(plannedStart, startOK),
			PlannedEndDate:     optionalWorkItemTime(plannedEnd, endOK),
			EstimatedWorkHours: estimatedWorkHours,
			CustomFields:       customValues,
			Timezone:           location.String(),
		}); err != nil {
			return &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
		}
		return nil
	}

	id, ok := parseWorkItemID(idText)
	if !ok || id <= 0 {
		return &workItemImportRowError{message: lib_i18n.Show(lang, `workitem_import_id_invalid`), skip: true}
	}
	current, err := GetRequirementById(adminId, wsId, cast.ToString(id))
	if err != nil {
		return &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
	}
	if len(current) == 0 {
		return &workItemImportRowError{message: lib_i18n.Show(lang, `workitem_import_id_not_current_project`), skip: true}
	}
	if current[`type`] != typeKey {
		return &workItemImportRowError{message: lib_i18n.Show(lang, `workitem_import_type_mismatch`), skip: true}
	}
	updates := map[string]any{}
	if title := value(`title`); title != `` {
		updates[`title`] = title
	}
	if description := value(`description`); description != `` {
		updates[`description`] = description
		updates[`description_mode`] = define.DescriptionModeRichText
	}
	if value(`status`) != `` && statusID > 0 {
		updates[`status_id`] = statusID
	}
	if value(`priority`) != `` && priorityID > 0 {
		updates[`priority`] = priorityID
	}
	if len(assigneeIDs) > 0 {
		updates[`assignee_ids`] = assigneeIDs
	}
	if value(`version`) != `` && versionID > 0 {
		updates[`version_id`] = versionID
	}
	if value(`planned_start_date`) != `` && startOK {
		updates[`planned_start_date`] = plannedStart
	}
	if value(`planned_end_date`) != `` && endOK {
		updates[`planned_end_date`] = plannedEnd
	}
	if value(`estimated_work_hours`) != `` {
		updates[`work_hours`] = estimatedWorkHours
	}
	nextStart := cast.ToInt64(current[`planned_start_date`])
	nextEnd := cast.ToInt64(current[`planned_end_date`])
	if planned, ok := updates[`planned_start_date`]; ok {
		nextStart = cast.ToInt64(planned)
	}
	if planned, ok := updates[`planned_end_date`]; ok {
		nextEnd = cast.ToInt64(planned)
	}
	if err = validate.ValidatePlannedDateRange(nextStart, nextEnd); err != nil {
		return &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
	}
	if dryRun {
		if err = ValidateCustomFieldValues(adminId, wsId, typeKey, customValues, false); err != nil {
			return &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
		}
		return nil
	}
	if err = UpdateRequirement(adminId, wsId, cast.ToString(id), updates, customValues, userId, nil); err != nil {
		return &workItemImportRowError{message: localizedWorkItemImportError(err, lang)}
	}
	return nil
}

func localizedWorkItemImportError(err error, lang string) string {
	switch {
	case errors.Is(err, ErrPriorityNotFound):
		return lib_i18n.Show(lang, define.MsgPriorityNotFound)
	case errors.Is(err, ErrStatusNotFound):
		return lib_i18n.Show(lang, define.MsgStatusNotFound)
	case errors.Is(err, ErrVersionNotFound):
		return lib_i18n.Show(lang, define.MsgVersionNotFound)
	case errors.Is(err, ErrRequirementRelationInvalid):
		return lib_i18n.Show(lang, define.MsgRequirementRelationInvalid)
	case errors.Is(err, validate.ErrPlannedDateRangeInvalid):
		return lib_i18n.Show(lang, define.MsgPlannedDateRangeInvalid)
	case errors.Is(err, validate.ErrRequirementWorkHoursInvalid):
		return lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid)
	case errors.Is(err, ErrCustomFieldValueInvalid):
		return CustomFieldValidationMessage(lang, err)
	case errors.Is(err, ErrRequirementAssigneesInvalid):
		return lib_i18n.Show(lang, define.MsgParamInvalid)
	default:
		var quotaErr *QuotaExceededError
		if errors.As(err, &quotaErr) {
			return lib_i18n.Show(lang, define.MsgQuotaExceeded)
		}
		return lib_i18n.Show(lang, define.MsgError)
	}
}

func parseWorkItemCustomValues(row []string, columns map[int]string, fields map[int]CustomFieldModel, location *time.Location) map[string]any {
	values := map[string]any{}
	for index, key := range columns {
		if !strings.HasPrefix(key, `custom:`) {
			continue
		}
		fieldID := cast.ToInt(strings.TrimPrefix(key, `custom:`))
		field, ok := fields[fieldID]
		if !ok {
			continue
		}
		raw := strings.TrimSpace(workItemCell(row, index))
		if raw == `` {
			continue
		}
		switch field.FieldType {
		case `number`:
			if number, err := strconv.ParseFloat(raw, 64); err == nil {
				values[cast.ToString(fieldID)] = number
			}
		case `date`:
			// Custom date fields are uniformly stored as YYYY-MM-DD strings, consistent with the format saved by frontend editing,
			// and tolerate slash/dot notations and date serial numbers produced by Excel auto-conversion.
			if ts, ok := parseWorkItemTime(raw, location); ok {
				values[cast.ToString(fieldID)] = time.Unix(ts, 0).In(location).Format(`2006-01-02`)
			}
		case `select_single`:
			if option := findWorkItemCustomOption(field, raw); option != `` {
				values[cast.ToString(fieldID)] = option
			}
		case `select_multi`:
			selected := []string{}
			seen := map[string]bool{}
			for _, name := range strings.Split(raw, `,`) {
				if option := findWorkItemCustomOption(field, strings.TrimSpace(name)); option != `` && !seen[option] {
					seen[option] = true
					selected = append(selected, option)
				}
			}
			if len(selected) > 0 {
				values[cast.ToString(fieldID)] = selected
			}
		default:
			values[cast.ToString(fieldID)] = raw
		}
	}
	return values
}

func findWorkItemCustomOption(field CustomFieldModel, value string) string {
	normalized := normalizeWorkItemLookup(value)
	for _, option := range field.Options {
		if normalizeWorkItemLookup(option.Name) == normalized || normalizeWorkItemLookup(option.Key) == normalized {
			return option.Key
		}
	}
	return ``
}

// readWorkItemColumnMap resolves each Excel column index to a field key.
//
// Two rules are enforced here rather than left to the callers:
//  1. Read-only columns (created_at / created_by / planning's related items) are dropped, so an
//     export can be edited and re-imported without those columns writing anything back.
//  2. Columns absent from the current layout are dropped. The meta sheet records the layout at
//     export time, so a field disabled after that export would otherwise still be writable through
//     import - an invisible change to a field the project has switched off.
func readWorkItemColumnMap(file *excelize.File, columns []WorkItemExcelColumn, typeKey string) (map[int]string, error) {
	allowed := make(map[string]bool, len(columns))
	for _, column := range columns {
		if !column.ReadOnly {
			allowed[column.Key] = true
		}
	}
	columnMap := map[int]string{}
	if index, err := file.GetSheetIndex(workItemMetaSheet); err == nil && index >= 0 {
		rows, rowsErr := file.GetRows(workItemMetaSheet)
		if rowsErr != nil {
			return nil, rowsErr
		}
		for _, row := range rows[1:] {
			if len(row) < 5 || row[4] != typeKey {
				continue
			}
			column := cast.ToInt(row[0]) - 1
			if column >= 0 && allowed[row[1]] {
				columnMap[column] = row[1]
			}
		}
	}
	if len(columnMap) > 0 {
		return columnMap, nil
	}
	sheetName := firstWorkItemSheet(file)
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return columnMap, nil
	}
	labels := make(map[string]string, len(columns))
	for _, column := range columns {
		if !column.ReadOnly {
			labels[normalizeWorkItemLookup(column.Label)] = column.Key
		}
	}
	headerRow := findWorkItemHeaderRow(rows, columns)
	for index, header := range rows[headerRow] {
		if key := labels[normalizeWorkItemLookup(header)]; key != `` {
			columnMap[index] = key
		}
	}
	return columnMap, nil
}

// findWorkItemHeaderRow is compatible with templates that include instructions, old export files, and new export files that keep only the header.
func findWorkItemHeaderRow(rows [][]string, columns []WorkItemExcelColumn) int {
	if len(rows) < 2 {
		return 0
	}
	labels := make(map[string]bool, len(columns))
	for _, column := range columns {
		labels[normalizeWorkItemLookup(column.Label)] = true
	}
	matchCount := func(row []string) int {
		count := 0
		for _, value := range row {
			if labels[normalizeWorkItemLookup(value)] {
				count++
			}
		}
		return count
	}
	if matchCount(rows[1]) > matchCount(rows[0]) {
		return 1
	}
	return 0
}

func firstWorkItemSheet(file *excelize.File) string {
	for _, name := range file.GetSheetList() {
		if name != workItemMetaSheet {
			return name
		}
	}
	return ``
}

func findWorkItemColumn(columns map[int]string, key string) int {
	for index, columnKey := range columns {
		if columnKey == key {
			return index
		}
	}
	return -1
}

func workItemCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ``
	}
	return row[index]
}

func workItemRowEmpty(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != `` {
			return false
		}
	}
	return true
}

func workItemLookupValue(values map[string]int, raw string) int {
	return values[normalizeWorkItemLookup(raw)]
}

// workItemLookupValues resolves comma-separated lookup names (e.g. multiple assignees) into ids.
func workItemLookupValues(values map[string]int, raw string) []int {
	ids := []int{}
	seen := map[int]bool{}
	for _, part := range strings.Split(raw, `,`) {
		if id := workItemLookupValue(values, strings.TrimSpace(part)); id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func normalizeWorkItemLookup(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseWorkItemID(value string) (int, bool) {
	if number, err := strconv.Atoi(value); err == nil {
		return number, true
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number != float64(int(number)) {
		return 0, false
	}
	return int(number), true
}

func loadWorkItemLocation(timezone string) (*time.Location, error) {
	if strings.TrimSpace(timezone) == `` {
		return time.UTC, nil
	}
	return time.LoadLocation(timezone)
}

// parseWorkItemTime parses a work item import date value. It tolerates multiple notations produced by Excel auto-conversion
// (e.g. 2026-01-01, 2026/01/01, 2026/1/1, 2026.1.1) and optional time parts;
// a plain number is parsed as an Excel date serial number (1900 date system).
func parseWorkItemTime(value string, location *time.Location) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == `` {
		return 0, false
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil && number > 0 {
		if parsed, timeErr := excelize.ExcelDateToTime(number, false); timeErr == nil {
			// The serial number carries no timezone info; interpret it as local midnight in the import timezone,
			// consistent with the string-parsing result.
			return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, location).Unix(), true
		}
	}
	formats := []string{
		`2006-01-02 15:04:05`,
		`2006-01-02`,
		`2006-1-2 15:04:05`,
		`2006-1-2`,
		`2006/01/02 15:04:05`,
		`2006/01/02`,
		`2006/1/2 15:04:05`,
		`2006/1/2`,
		`2006.01.02`,
		`2006.1.2`,
	}
	for _, format := range formats {
		if parsed, err := time.ParseInLocation(format, value, location); err == nil {
			return parsed.Unix(), true
		}
	}
	return 0, false
}

func optionalWorkItemTime(value int64, ok bool) int64 {
	if ok {
		return value
	}
	return 0
}

func formatWorkItemTime(value int64, location *time.Location) string {
	if value <= 0 {
		return ``
	}
	return time.Unix(value, 0).In(location).Format(`2006-01-02 15:04:05`)
}

func formatWorkItemDate(value int64, location *time.Location) string {
	if value <= 0 {
		return ``
	}
	return time.Unix(value, 0).In(location).Format(`2006-01-02`)
}

// isWorkItemDateColumn reports whether a column is a date column (built-in planned start/end time or a custom date field);
// such columns must be set to text format in the import template to prevent Excel from auto-converting date notations.
func isWorkItemDateColumn(column WorkItemExcelColumn) bool {
	return column.Key == `planned_start_date` || column.Key == `planned_end_date` ||
		(column.CustomField != nil && column.CustomField.FieldType == `date`)
}
