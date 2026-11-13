// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/xuri/excelize/v2"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// ======================== Enterprise Member Excel Import/Export ========================

const (
	// memberExcelTypeKey tags the hidden meta sheet so the work-item column-map machinery can be reused.
	memberExcelTypeKey = `member`
	memberExcelMaxRows = 10000
	// MemberImportDefaultPassword is used when the password column is empty for a new account
	// (also documented in the template column note).
	MemberImportDefaultPassword = `goteams123`
)

// memberExcelLangs lists the languages whose role display names are matched on import.
var memberExcelLangs = []string{`zh-CN`, `en-US`}

// MemberImportResult returns the member import summary (JSON shape mirrors the work-item import).
type MemberImportResult struct {
	Total   int                   `json:"total"`
	Created int                   `json:"created"`
	Updated int                   `json:"updated"`
	Skipped int                   `json:"skipped"`
	Failed  int                   `json:"failed"`
	Errors  []WorkItemImportError `json:"errors"`
}

// MemberImportPreview returns the pre-parsed member file counts without writing to the database.
type MemberImportPreview struct {
	Total      int `json:"total"`
	Importable int `json:"importable"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

// buildMemberExcelColumns assembles the member Excel columns (reuses the work-item column type
// so the shared header/meta/column-map helpers apply as-is).
func buildMemberExcelColumns(lang string, includeAuditColumns bool) []WorkItemExcelColumn {
	description := func(key string) string {
		if key == `password` {
			return fmt.Sprintf(lib_i18n.Show(lang, `member_excel_desc_password`), MemberImportDefaultPassword)
		}
		return lib_i18n.Show(lang, `member_excel_desc_`+key)
	}
	columns := []WorkItemExcelColumn{
		{Key: `name`, Label: lib_i18n.Show(lang, `member_excel_col_name`), Description: description(`name`)},
		{Key: `username`, Label: lib_i18n.Show(lang, `member_excel_col_username`), Description: description(`username`)},
		{Key: `password`, Label: lib_i18n.Show(lang, `member_excel_col_password`), Description: description(`password`)},
		{Key: `phone`, Label: lib_i18n.Show(lang, `member_excel_col_phone`), Description: description(`phone`)},
		{Key: `email`, Label: lib_i18n.Show(lang, `member_excel_col_email`), Description: description(`email`)},
		{Key: `role`, Label: lib_i18n.Show(lang, `member_excel_col_role`), Description: description(`role`)},
		{Key: `department`, Label: lib_i18n.Show(lang, `member_excel_col_department`), Description: description(`department`)},
	}
	if includeAuditColumns {
		columns = append(columns, WorkItemExcelColumn{
			Key: `created_at`, Label: lib_i18n.Show(lang, `member_excel_col_created_at`),
			Description: description(`created_at`), ReadOnly: true,
		})
	}
	return columns
}

// BuildEnterpriseMemberWorkbook generates the member import template or the full member export.
func BuildEnterpriseMemberWorkbook(adminId int, lang, timezone string, templateOnly bool) ([]byte, error) {
	columns := buildMemberExcelColumns(lang, !templateOnly)
	location, err := loadWorkItemLocation(timezone)
	if err != nil {
		return nil, err
	}
	file := excelize.NewFile()
	sheetName := lib_i18n.Show(lang, `member_excel_sheet`)
	if err = file.SetSheetName(`Sheet1`, sheetName); err != nil {
		return nil, err
	}
	if err = writeWorkItemWorkbookHeader(file, sheetName, columns, templateOnly); err != nil {
		return nil, err
	}
	if err = writeWorkItemWorkbookMeta(file, columns, memberExcelTypeKey); err != nil {
		return nil, err
	}
	if !templateOnly {
		model, listErr := ListEnterpriseMembers(ListEnterpriseMembersModel{AdminId: adminId})
		if listErr != nil {
			return nil, listErr
		}
		roleNames, rolesErr := enterpriseRoleDisplayNames(adminId, lang)
		if rolesErr != nil {
			return nil, rolesErr
		}
		departmentPaths, pathErr := DepartmentPathNames(adminId)
		if pathErr != nil {
			return nil, pathErr
		}
		for rowIndex, member := range model.Items {
			paths := make([]string, 0, len(member.DepartmentIds))
			for _, departmentId := range member.DepartmentIds {
				if path := departmentPaths[departmentId]; path != `` {
					paths = append(paths, path)
				}
			}
			values := []any{
				member.Name, member.Username, ``, member.Phone, member.Email,
				roleNames[member.RoleId], strings.Join(paths, `,`),
				formatWorkItemTime(member.CreatedAt, location),
			}
			for colIndex, value := range values {
				cell, cellErr := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
				if cellErr != nil {
					return nil, cellErr
				}
				if err = file.SetCellValue(sheetName, cell, value); err != nil {
					return nil, err
				}
			}
		}
	}
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// enterpriseRoleDisplayNames returns roleId -> localized display name for export.
func enterpriseRoleDisplayNames(adminId int, lang string) (map[int]string, error) {
	m := msql.Model(define.TableRoles, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`scope`, permission.ScopeEnterprise},
	}).Field(`id, name`).Select()
	if err != nil {
		logs.Error("list enterprise role names error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	result := make(map[int]string, len(rows))
	for _, row := range rows {
		result[cast.ToInt(row[`id`])] = lib_i18n.ShowStored(lang, row[`name`])
	}
	return result, nil
}

// memberImportLookups bundles the reference data a member import needs.
type memberImportLookups struct {
	membersByUsername map[string]EnterpriseMemberModel
	roleIdsByName     map[string]int
	defaultRoleId     int
	adminId           int
	operatorUserId    int
}

func buildMemberImportLookups(adminId, operatorUserId int) (memberImportLookups, error) {
	lookups := memberImportLookups{
		membersByUsername: map[string]EnterpriseMemberModel{},
		roleIdsByName:     map[string]int{},
		adminId:           adminId,
		operatorUserId:    operatorUserId,
	}
	model, err := ListEnterpriseMembers(ListEnterpriseMembersModel{AdminId: adminId})
	if err != nil {
		return lookups, err
	}
	for _, member := range model.Items {
		lookups.membersByUsername[normalizeWorkItemLookup(member.Username)] = member
	}
	m := msql.Model(define.TableRoles, define.Postgres)
	roles, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`scope`, permission.ScopeEnterprise},
	}).Field(`id, name`).Order(`id ASC`).Select()
	if err != nil {
		logs.Error("list enterprise roles for import error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return lookups, err
	}
	for _, row := range roles {
		roleId := cast.ToInt(row[`id`])
		name := row[`name`]
		lookups.roleIdsByName[normalizeWorkItemLookup(name)] = roleId
		for _, lang := range memberExcelLangs {
			lookups.roleIdsByName[normalizeWorkItemLookup(lib_i18n.ShowStored(lang, name))] = roleId
		}
		if name == lib_i18n.WrapKey(`perm_role_enterprise_member`) {
			lookups.defaultRoleId = roleId
		}
	}
	// Fallback: if the seeded member role is missing, use the last role so import still works.
	if lookups.defaultRoleId == 0 && len(roles) > 0 {
		lookups.defaultRoleId = cast.ToInt(roles[len(roles)-1][`id`])
	}
	return lookups, nil
}

// ImportEnterpriseMembers creates or updates members row by row: matching usernames update, new usernames create.
func ImportEnterpriseMembers(reader io.Reader, adminId int, lang string, operatorUserId int) (MemberImportResult, error) {
	result := MemberImportResult{Errors: []WorkItemImportError{}}
	rows, columnMap, err := readMemberExcelRows(reader, lang)
	if err != nil {
		return result, err
	}
	if rows == nil {
		return result, nil
	}
	lookups, err := buildMemberImportLookups(adminId, operatorUserId)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Total++
		username := strings.TrimSpace(workItemCell(row.cells, findWorkItemColumn(columnMap, `username`)))
		rowErr, created := importMemberRow(row.cells, columnMap, &lookups, lang, false)
		if rowErr == nil {
			if created {
				result.Created++
			} else {
				result.Updated++
			}
			continue
		}
		result.Failed++
		result.Errors = append(result.Errors, WorkItemImportError{Row: row.number, Id: username, Reason: rowErr.message})
	}
	return result, nil
}

// PreviewEnterpriseMembers pre-parses the member file and counts importable rows without writing.
func PreviewEnterpriseMembers(reader io.Reader, adminId int, lang string, operatorUserId int) (MemberImportPreview, error) {
	result := MemberImportPreview{}
	rows, columnMap, err := readMemberExcelRows(reader, lang)
	if err != nil {
		return result, err
	}
	if rows == nil {
		return result, nil
	}
	lookups, err := buildMemberImportLookups(adminId, operatorUserId)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Total++
		if rowErr, _ := importMemberRow(row.cells, columnMap, &lookups, lang, true); rowErr == nil {
			result.Importable++
		} else {
			result.Failed++
		}
	}
	return result, nil
}

type memberExcelRow struct {
	number int
	cells  []string
}

// readMemberExcelRows opens the workbook and returns the non-empty data rows with the column mapping.
// A nil rows result with nil error means the sheet holds no data rows.
func readMemberExcelRows(reader io.Reader, lang string) ([]memberExcelRow, map[int]string, error) {
	file, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = file.Close() }()
	columns := buildMemberExcelColumns(lang, true)
	columnMap, err := readWorkItemColumnMap(file, columns, memberExcelTypeKey)
	if err != nil {
		return nil, nil, err
	}
	sheetName := firstWorkItemSheet(file)
	if sheetName == `` {
		return nil, nil, fmt.Errorf("member sheet not found")
	}
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, nil, err
	}
	headerRow := findWorkItemHeaderRow(rows, columns)
	dataStartRow := headerRow + 1
	if len(rows) <= dataStartRow {
		return nil, columnMap, nil
	}
	if len(rows)-dataStartRow > memberExcelMaxRows {
		return nil, nil, fmt.Errorf("too many rows: %d", len(rows)-dataStartRow)
	}
	result := make([]memberExcelRow, 0, len(rows)-dataStartRow)
	for index, row := range rows[dataStartRow:] {
		if workItemRowEmpty(row) {
			continue
		}
		result = append(result, memberExcelRow{number: index + dataStartRow + 1, cells: row})
	}
	return result, columnMap, nil
}

// importMemberRow validates one row and (unless dryRun) creates or updates the member.
// The second return value reports whether the row created a new member.
func importMemberRow(row []string, columnMap map[int]string, lookups *memberImportLookups, lang string, dryRun bool) (*workItemImportRowError, bool) {
	value := func(key string) string {
		return strings.TrimSpace(workItemCell(row, findWorkItemColumn(columnMap, key)))
	}
	name := value(`name`)
	username := validate.NormalizeEmail(value(`username`))
	password := value(`password`)
	phone := value(`phone`)
	email := validate.NormalizeEmail(value(`email`))
	roleText := value(`role`)
	departmentText := value(`department`)

	if username == `` {
		return &workItemImportRowError{message: lib_i18n.Show(lang, `member_import_username_required`)}, false
	}
	existing, exists := lookups.membersByUsername[normalizeWorkItemLookup(username)]
	if !exists && name == `` {
		return &workItemImportRowError{message: lib_i18n.Show(lang, `member_import_name_required`)}, false
	}
	if err := validate.ValidateAccount(`username`, username); err != nil {
		return &workItemImportRowError{message: lib_i18n.Show(lang, define.MsgParamInvalid)}, false
	}
	if name != `` {
		if err := validate.ValidateProfile(name); err != nil {
			return &workItemImportRowError{message: lib_i18n.Show(lang, define.MsgParamInvalid)}, false
		}
	}
	if err := validate.ValidateMemberPhone(phone); err != nil {
		return &workItemImportRowError{message: lib_i18n.Show(lang, define.MsgParamInvalid)}, false
	}
	if err := validate.ValidateMemberEmail(email); err != nil {
		return &workItemImportRowError{message: lib_i18n.Show(lang, define.MsgParamInvalid)}, false
	}
	if password != `` {
		if err := validate.IsValidPassword(password); err != nil {
			return &workItemImportRowError{message: lib_i18n.Show(lang, `member_import_password_invalid`)}, false
		}
	}

	roleId := 0
	if roleText == `` {
		if exists {
			roleId = existing.RoleId
		} else {
			roleId = lookups.defaultRoleId
		}
	} else {
		roleId = lookups.roleIdsByName[normalizeWorkItemLookup(roleText)]
		if roleId == 0 {
			return &workItemImportRowError{message: lib_i18n.Show(lang, `member_import_role_not_found`)}, false
		}
	}

	departmentPaths, pathErr := parseMemberDepartmentPaths(departmentText)
	if pathErr != nil {
		return &workItemImportRowError{message: localizedMemberImportError(pathErr, lang)}, false
	}

	if dryRun {
		if !exists {
			taken, err := enterpriseUsernameExists(username, 0)
			if err != nil {
				return &workItemImportRowError{message: lib_i18n.Show(lang, define.MsgError)}, false
			}
			if taken {
				return &workItemImportRowError{message: lib_i18n.Show(lang, define.MsgUserExist)}, false
			}
		}
		// Reuse the real write path's exclusion rule so preview matches what the import would do.
		excludedUserId := 0
		if exists {
			excludedUserId = existing.Id
		}
		emailTaken, err := UserEmailExist(email, excludedUserId)
		if err != nil {
			return &workItemImportRowError{message: lib_i18n.Show(lang, define.MsgError)}, false
		}
		if emailTaken {
			return &workItemImportRowError{message: lib_i18n.Show(lang, define.MsgEmailExists)}, false
		}
		return nil, !exists
	}

	departmentIds := make([]int, 0, len(departmentPaths))
	for _, path := range departmentPaths {
		departmentId, err := EnsureDepartmentPath(lookups.adminId, path)
		if err != nil {
			return &workItemImportRowError{message: localizedMemberImportError(err, lang)}, false
		}
		departmentIds = append(departmentIds, departmentId)
	}

	if !exists {
		if password == `` {
			password = MemberImportDefaultPassword
		}
		model, err := CreateEnterpriseMember(CreateEnterpriseMemberModel{
			AdminId: lookups.adminId, Username: username, Password: password, Name: name,
			Phone: phone, Email: email, RoleId: roleId, DepartmentIds: departmentIds,
		})
		if err != nil {
			return &workItemImportRowError{message: localizedMemberImportError(err, lang)}, false
		}
		lookups.membersByUsername[normalizeWorkItemLookup(username)] = EnterpriseMemberModel{
			Id: model.UserId, Username: username, Name: name, Phone: phone, Email: email, RoleId: roleId,
		}
		return nil, true
	}

	// Existing member: empty cells keep current values, matching the work-item import semantics.
	updated := existing
	if name != `` {
		updated.Name = name
	}
	if phone != `` {
		updated.Phone = phone
	}
	// An empty cell keeps the current value, so a blank email column cannot unbind a member.
	if email != `` {
		updated.Email = email
	}
	err := UpdateEnterpriseMember(UpdateEnterpriseMemberModel{
		AdminId: lookups.adminId, OperatorUserId: lookups.operatorUserId, UserId: existing.Id,
		Username: existing.Username, Name: updated.Name, Phone: updated.Phone, Email: updated.Email,
		RoleId: roleId, DepartmentIds: departmentIds, UpdateDepartments: departmentText != ``,
	})
	if err != nil {
		return &workItemImportRowError{message: localizedMemberImportError(err, lang)}, false
	}
	if password != `` {
		if err = ResetEnterpriseMemberPassword(ResetEnterpriseMemberPasswordModel{
			AdminId: lookups.adminId, OperatorUserId: lookups.operatorUserId,
			TargetUserId: existing.Id, NewPassword: password,
		}); err != nil {
			return &workItemImportRowError{message: localizedMemberImportError(err, lang)}, false
		}
	}
	lookups.membersByUsername[normalizeWorkItemLookup(username)] = updated
	return nil, false
}

// parseMemberDepartmentPaths parses "产品部/设计组,研发部" into path segment lists.
// Chinese commas are tolerated; each segment is validated for emptiness, length and depth.
func parseMemberDepartmentPaths(text string) ([][]string, error) {
	text = strings.ReplaceAll(text, `，`, `,`)
	paths := make([][]string, 0, 4)
	for _, pathText := range strings.Split(text, `,`) {
		pathText = strings.TrimSpace(pathText)
		if pathText == `` {
			continue
		}
		segments := strings.Split(pathText, `/`)
		if len(segments) > DepartmentMaxDepth {
			return nil, ErrDepartmentDepthExceeded
		}
		names := make([]string, 0, len(segments))
		for _, segment := range segments {
			name, err := NormalizeDepartmentName(segment)
			if err != nil {
				return nil, err
			}
			names = append(names, name)
		}
		paths = append(paths, names)
	}
	return paths, nil
}

func localizedMemberImportError(err error, lang string) string {
	var quotaErr *QuotaExceededError
	switch {
	case errors.As(err, &quotaErr):
		return lib_i18n.Show(lang, define.MsgQuotaExceeded)
	case errors.Is(err, ErrUserUsernameExists):
		return lib_i18n.Show(lang, define.MsgUserExist)
	case errors.Is(err, ErrUserEmailExists):
		return lib_i18n.Show(lang, define.MsgEmailExists)
	case errors.Is(err, ErrEnterpriseAdminProtected):
		return lib_i18n.Show(lang, `member_import_admin_protected`)
	case errors.Is(err, ErrEnterpriseRoleNotFound):
		return lib_i18n.Show(lang, `member_import_role_not_found`)
	case errors.Is(err, ErrDepartmentDepthExceeded):
		return lib_i18n.Show(lang, define.MsgDepartmentDepthExceeded)
	case errors.Is(err, ErrDepartmentNameExists), errors.Is(err, ErrDepartmentInvalid), errors.Is(err, ErrDepartmentNotFound):
		return lib_i18n.Show(lang, define.MsgDepartmentInvalid)
	case errors.Is(err, ErrEnterpriseMemberNotFound):
		return lib_i18n.Show(lang, define.MsgUserNotFound)
	default:
		return lib_i18n.Show(lang, define.MsgError)
	}
}
