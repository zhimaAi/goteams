// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// templatePackageFilename is the fixed download name: an operator drops the file straight onto
// assets/template/template.tar.gz, so it must never carry a timestamp or an admin id.
const templatePackageFilename = `template.tar.gz`

// ExportTemplatePackageModel is the export use case input.
type ExportTemplatePackageModel struct {
	AdminID int // account being exported; must be the configured template account
}

// ExportTemplatePackageResult is the finished package plus the asset counters of the run.
type ExportTemplatePackageResult struct {
	Filename          string
	Data              []byte
	AssetCount        int // stored references the package carries
	MissingAssetCount int // stored references that could not be exported
}

// templateExportWorkspaceFields is the workspaces SELECT of the export. It is wider than the
// project list because two consumers read the same rows: toWorkspaceModel parses id,
// active_version_id, created_by, created_at and updated_at strictly (a value that is not a
// number is a conversion error, and a NULL column arrives as the empty string, hence the
// COALESCE wrappers), and the package loader cross-checks admin_id, ended_at and sort_order
// against the manifest.
const templateExportWorkspaceFields = `id, admin_id, workspace_type,
	COALESCE((SELECT name FROM ` + define.TableWorkspaceTypes + ` WHERE type_key = workspaces.workspace_type), '') AS workspace_type_name,
	name, slug, description, color, icon, sort_order,
	COALESCE(ended_at, 0) AS ended_at, COALESCE(active_version_id, 0) AS active_version_id,
	created_by, created_at, updated_at`

// templatePoolTemplateFields are the pool template columns the copy step consumes through
// TemplateSource.PoolTemplate; the id is the lookup key.
var templatePoolTemplateFields = []string{`id`, `type_key`, `name`, `title`, `description`,
	`description_mode`, `fields_json`}

// templateDerivedSQLAlias names the SELECT alias a SQL-side derivation arrives under. A derived
// key without an entry here is produced by the exporter itself (roles permissions, knowledge
// source assets), because no single expression can compute it.
var templateDerivedSQLAlias = map[string]string{
	TemplateDerivedVersionShared:   `shared_flag`,
	TemplateDerivedIssueItemFields: `fields_json`,
}

// ExportTemplatePackage packages every copyable configuration, business row and image of the
// template account into one offline package. Private deployments have no template account in
// their own database, so this file is how the template reaches them: it is committed and then
// distributed with the code, which is why credentials are stripped and the result is fed back
// through the reader before it is handed out.
func ExportTemplatePackage(model ExportTemplatePackageModel) (ExportTemplatePackageResult, error) {
	// The endpoint passes the configured account itself; asserting it here keeps a future caller
	// from exporting an arbitrary tenant through the template endpoint.
	if model.AdminID <= 0 || model.AdminID != GetTemplateAdminId() {
		return ExportTemplatePackageResult{}, fmt.Errorf(`%w: admin %d is not the configured template account`,
			ErrTemplateExportForbidden, model.AdminID)
	}
	exporter := newTemplateExporter(model.AdminID)
	if err := exporter.collect(); err != nil {
		return ExportTemplatePackageResult{}, err
	}
	data, err := exporter.pack()
	if err != nil {
		return ExportTemplatePackageResult{}, err
	}
	// A package this binary's own reader rejects would only surface on the customer side as a
	// broken "create project from template", so the export fails here instead.
	if _, err := parseTemplatePackage(data); err != nil {
		return ExportTemplatePackageResult{}, fmt.Errorf(`template export produced an unreadable package: %w`, err)
	}
	logs.Info(`template export: admin [%d] workspaces [%d] package bytes [%d] assets [%d] missing assets [%d]`,
		model.AdminID, len(exporter.workspaceIDs), len(data), len(exporter.assets), exporter.missing)
	return ExportTemplatePackageResult{
		Filename:          templatePackageFilename,
		Data:              data,
		AssetCount:        len(exporter.assets),
		MissingAssetCount: exporter.missing,
	}, nil
}

// templateExporter gathers a whole package in memory before writing it, so the size limits are
// enforced once on the finished archive instead of mid-collection.
type templateExporter struct {
	adminID      int
	workspaces   []msql.Params
	workspaceIDs []int
	sources      map[int]map[string][]msql.Params // workspace id -> entry base name -> rows
	poolIDs      map[int]bool
	poolRows     []msql.Params
	assets       map[string]templateAssetIndexRow // asset ref (package path) -> index row
	assetBlobs   map[string][]byte                // asset entry name -> bytes
	assetRefs    map[string]string                // template storage value -> package asset ref
	sourceAssets map[int]map[int]string           // workspace id -> document id -> attachment stored value
	missing      int
	scanner      *TemplateConfigCopier
}

func newTemplateExporter(adminID int) *templateExporter {
	return &templateExporter{
		adminID:      adminID,
		sources:      make(map[int]map[string][]msql.Params),
		poolIDs:      make(map[int]bool),
		assets:       make(map[string]templateAssetIndexRow),
		assetBlobs:   make(map[string][]byte),
		assetRefs:    make(map[string]string),
		sourceAssets: make(map[int]map[int]string),
		// The content scan of the copy flow is reused verbatim: what the exporter registers in
		// the index and what the copy later looks up with ReadAsset must be the same set.
		scanner: &TemplateConfigCopier{templateAdminId: adminID},
	}
}

// collect reads the live template account: projects, their sources, the pool templates they
// reference, and finally the storage objects those rows reference.
func (e *templateExporter) collect() error {
	if err := e.collectWorkspaces(); err != nil {
		return err
	}
	for _, wsID := range e.workspaceIDs {
		if err := e.collectWorkspace(wsID); err != nil {
			return err
		}
	}
	if err := e.collectPoolTemplates(); err != nil {
		return err
	}
	candidates, err := e.assetCandidates()
	if err != nil {
		return err
	}
	for _, stored := range candidates {
		if err := e.exportAsset(stored); err != nil {
			return err
		}
	}
	e.rewriteAssetReferences()
	e.applyKnowledgeSourceAssets()
	return nil
}

// collectWorkspaces reads the template account's unfinished projects in the order the template
// list uses (manual sort first, newest project before older ones).
func (e *templateExporter) collectWorkspaces() error {
	rows, err := msql.RawValues(define.Postgres, `SELECT `+templateExportWorkspaceFields+` FROM `+define.TableWorkspaces+`
		WHERE admin_id = $1 AND COALESCE(ended_at, 0) = 0
		ORDER BY sort_order ASC, id DESC`, nil, e.adminID)
	if err != nil {
		logs.Error(`template export: load template workspaces err:%s`, err.Error())
		return err
	}
	// 历史模板账号不回填类型，重新导出时保留分发包中已明确标注的类型元数据。
	if pkg, loadErr := LoadTemplatePackage(); loadErr == nil {
		if pkg.TemplateAdminID() == e.adminID {
			types := make(map[string]WorkspaceTypeModel)
			for _, source := range pkg.Workspaces() {
				types[source[`id`]] = WorkspaceTypeModel{Key: source[`workspace_type`], Name: source[`workspace_type_name`]}
			}
			for _, row := range rows {
				if row[`workspace_type`] == "" {
					row[`workspace_type`] = types[row[`id`]].Key
					row[`workspace_type_name`] = types[row[`id`]].Name
				}
			}
		}
	} else {
		logTemplatePackageUnavailable(`preserve workspace types on export`, loadErr)
	}
	e.workspaces = rows
	e.workspaceIDs = make([]int, 0, len(rows))
	for _, row := range rows {
		e.workspaceIDs = append(e.workspaceIDs, cast.ToInt(row[`id`]))
	}
	return nil
}

// collectWorkspace reads every declared source of one project, then applies the per-source
// work that a SELECT cannot do: dropping webhook actions from automation rules and the role
// permission list.
func (e *templateExporter) collectWorkspace(wsID int) error {
	sources := make(map[string][]msql.Params, len(templateTables))
	for _, table := range templateTables {
		rows, err := e.sourceRows(table, wsID)
		if err != nil {
			return err
		}
		switch table.Name {
		case SrcAutomationRules:
			exportable := rows[:0]
			for i := range rows {
				config, ok := stripAutomationWebhooks(rows[i][`config_json`])
				if !ok {
					continue
				}
				rows[i][`config_json`] = config
				exportable = append(exportable, rows[i])
			}
			rows = exportable
		case SrcRoles:
			if err := e.fillRolePermissions(rows); err != nil {
				return err
			}
		}
		sources[table.Name] = rows
	}
	e.sources[wsID] = sources
	return e.collectPoolReferences(wsID)
}

// sourceRows reads one source from the live database with the column list and order the package
// declares, so a package row is byte-identical to the row the live copy used to read.
func (e *templateExporter) sourceRows(table TemplateTable, wsID int) ([]msql.Params, error) {
	from := table.From
	if from == `` {
		from = table.Table
	}
	where := table.Where
	if where == `` {
		where = `admin_id = $1 AND workspace_id = $2`
	}
	selected := make([]string, 0, len(table.Columns)+len(table.DerivedSelect))
	selected = append(selected, table.Columns...)
	selected = append(selected, table.DerivedSelect...)
	// Bind order is the placeholder order the source spec assumes: $1 is the template account,
	// $2 the project, and Args fill $3 upward. Every raw fragment of templateTables uses both $1
	// and $2, so repeated placeholders may reuse them (PostgreSQL binds by number).
	args := make([]any, 0, 2+len(table.Args))
	args = append(args, e.adminID, wsID)
	for _, arg := range table.Args {
		args = append(args, arg)
	}
	order := table.OrderBy
	if order != `` {
		order = ` ` + order
	}
	rows, err := msql.RawValues(define.Postgres,
		`SELECT `+strings.Join(selected, `, `)+` FROM `+from+` WHERE `+where+order, nil, args...)
	if err != nil {
		logs.Error(`template export: load source [%s] of workspace [%d] err:%s`, table.Name, wsID, err.Error())
		return nil, err
	}
	return projectTemplateRows(table, rows)
}

// projectTemplateRows narrows a SELECT result to the package row shape: the declared columns
// under their bare name, plus the declared derived keys. Extra SELECT columns a subquery needs
// to keep its own order (the view union's branch and row_id) stay out, because the copy side
// reads rows by the column list it was written against.
func projectTemplateRows(table TemplateTable, raw []msql.Params) ([]msql.Params, error) {
	bare := make([]string, len(table.Columns))
	for i, column := range table.Columns {
		bare[i] = templateBareColumn(column)
		if !isTemplateColumnName(bare[i]) {
			return nil, fmt.Errorf(`template export: source %s declares an invalid column %q`, table.Name, column)
		}
	}
	rows := make([]msql.Params, 0, len(raw))
	for _, source := range raw {
		row := make(msql.Params, len(table.Columns)+len(table.Derived))
		for _, key := range bare {
			row[key] = source[key]
		}
		for _, key := range table.Derived {
			// NULL arrives as an empty string from the driver, which is also the package's
			// representation of "no value"; derived keys the exporter fills later start empty.
			if alias, ok := templateDerivedSQLAlias[key]; ok {
				row[key] = source[alias]
				continue
			}
			row[key] = ``
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// templateBareColumn strips the table qualifier of a SELECT column: the driver reports the field
// name alone, so `v.id` arrives as `id`.
func templateBareColumn(column string) string {
	if index := strings.LastIndex(column, `.`); index >= 0 {
		return column[index+1:]
	}
	return column
}

// isTemplateColumnName accepts the lower snake_case identifiers the package declares; it keeps a
// typo in templateTables from turning into a column of empty values.
func isTemplateColumnName(column string) bool {
	if column == `` {
		return false
	}
	for _, r := range column {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

// fillRolePermissions appends each role's permission keys as a JSON array string. The copy step
// used to re-query role_permissions per role; the package carries the result so the copy never
// touches the template tenant.
func (e *templateExporter) fillRolePermissions(rows []msql.Params) error {
	for _, row := range rows {
		permissions, err := msql.RawValues(define.Postgres, `SELECT permission_key FROM `+define.TableRolePermissions+`
			WHERE admin_id = $1 AND role_id = $2
			ORDER BY id ASC`, nil, e.adminID, cast.ToInt(row[`id`]))
		if err != nil {
			logs.Error(`template export: load permissions of role [%s] err:%s`, row[`id`], err.Error())
			return err
		}
		keys := make([]string, 0, len(permissions))
		for _, permission := range permissions {
			keys = append(keys, permission[`permission_key`])
		}
		encoded, err := json.Marshal(keys)
		if err != nil {
			logs.Error(`template export: encode permissions of role [%s] err:%s`, row[`id`], err.Error())
			return err
		}
		row[TemplateDerivedRolePermissions] = string(encoded)
	}
	return nil
}

// collectPoolReferences records the pool templates one project references. The invitation
// association rows fold their pool template into fields_json at export time and never carry the
// link column, so that reference has to be read from the live row.
func (e *templateExporter) collectPoolReferences(wsID int) error {
	for _, row := range e.sources[wsID][SrcWorkItemTemplateLinks] {
		e.addPoolID(cast.ToInt(row[`template_id`]))
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT template_id FROM `+define.TableWorkspaceIssueItemTemplates+`
		WHERE admin_id = $1 AND workspace_id = $2 AND template_id > 0
		ORDER BY id ASC`, nil, e.adminID, wsID)
	if err != nil {
		logs.Error(`template export: load issue invitation template links of workspace [%d] err:%s`, wsID, err.Error())
		return err
	}
	for _, row := range rows {
		e.addPoolID(cast.ToInt(row[`template_id`]))
	}
	return nil
}

func (e *templateExporter) addPoolID(templateID int) {
	if templateID > 0 {
		e.poolIDs[templateID] = true
	}
}

// collectPoolTemplates reads the referenced tenant-level pool templates once for the whole
// package: they are shared across projects and the copy looks them up by id.
func (e *templateExporter) collectPoolTemplates() error {
	if len(e.poolIDs) == 0 {
		return nil
	}
	ids := make([]int, 0, len(e.poolIDs))
	for templateID := range e.poolIDs {
		ids = append(ids, templateID)
	}
	sort.Ints(ids)
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, e.adminID)
	for i, templateID := range ids {
		placeholders = append(placeholders, fmt.Sprintf(`$%d`, i+2))
		args = append(args, templateID)
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT `+strings.Join(templatePoolTemplateFields, `, `)+`
		FROM `+define.TableWorkItemTemplates+`
		WHERE admin_id = $1 AND id IN (`+strings.Join(placeholders, `, `)+`)
		ORDER BY id ASC`, nil, args...)
	if err != nil {
		logs.Error(`template export: load pool templates err:%s`, err.Error())
		return err
	}
	// A link whose pool template is gone would be skipped by the copy too; keeping the dangling
	// id out of the package is the same degradation, recorded for the operator.
	referenced := make(map[int]bool, len(rows))
	for _, row := range rows {
		referenced[cast.ToInt(row[`id`])] = true
	}
	for _, templateID := range ids {
		if !referenced[templateID] {
			logs.Warning(`template export: referenced pool template [%d] has no row, dropped from the package`, templateID)
		}
	}
	e.poolRows = rows
	return nil
}

// assetCandidates lists every stored object the exported rows and content reference, in package
// order. All four reference classes have to be covered: the project avatars (workspaces.icon,
// shown on the create-project template cards), URLs inside rich text and markdown, the
// invitation theme images (which the live copy wrote into the destination without migrating the
// object), and the attachment backing a file-type knowledge document.
func (e *templateExporter) assetCandidates() ([]string, error) {
	candidates := make([]string, 0)
	seen := make(map[string]bool)
	add := func(stored string) {
		if stored != `` && !seen[stored] {
			seen[stored] = true
			candidates = append(candidates, stored)
		}
	}
	for _, row := range e.workspaces {
		add(row[`icon`])
	}
	for _, wsID := range e.workspaceIDs {
		sources := e.sources[wsID]
		for _, row := range sources[SrcIssueInvites] {
			add(row[`theme_image`])
		}
		for _, row := range sources[SrcIssueItemTemplates] {
			add(row[`theme_image`])
		}
		// Work item descriptions are scanned whatever their mode is, exactly like the copy step.
		for _, row := range sources[SrcWorkItems] {
			for _, stored := range e.scanner.storedURLsInContent(row[`description`]) {
				add(stored)
			}
		}
		for _, row := range sources[SrcKnowledgeDocContents] {
			if row[`content_mode`] == define.KnowledgeContentModeFile {
				// A file-mode document's content is never rewritten by the copy, so an URL in it
				// would keep pointing at the template account either way.
				continue
			}
			for _, stored := range e.scanner.storedURLsInContent(row[`content`]) {
				add(stored)
			}
		}
		if err := e.collectDocumentSources(wsID, sources[SrcKnowledgeDocContents]); err != nil {
			return nil, err
		}
		for _, stored := range e.sourceAssets[wsID] {
			add(stored)
		}
		// Dashboard card bodies and the project announcement are rich text, scanned exactly like
		// work item descriptions.
		for _, row := range sources[SrcDashboardCards] {
			for _, stored := range e.scanner.storedURLsInContent(row[`content`]) {
				add(stored)
			}
		}
		for _, row := range sources[SrcDashboardAnnouncements] {
			for _, stored := range e.scanner.storedURLsInContent(row[`content`]) {
				add(stored)
			}
		}
	}
	return candidates, nil
}

// collectDocumentSources resolves the ledger object behind each document's source attachment and
// remembers it per document so source_asset can be filled once the assets are known.
func (e *templateExporter) collectDocumentSources(wsID int, rows []msql.Params) error {
	byDocument := e.sourceAssets[wsID]
	for _, row := range rows {
		attachmentID := cast.ToInt(row[`source_attachment_id`])
		if attachmentID <= 0 {
			continue
		}
		ledger, err := msql.RawValues(define.Postgres, `SELECT stored_path FROM `+define.GetEnterpriseFileTable(e.adminID)+`
			WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 AND source_type = $4
			ORDER BY id ASC LIMIT 1`, nil, e.adminID, wsID, attachmentID, define.SourceTypeKnowledge)
		if err != nil {
			logs.Error(`template export: load knowledge source attachment [%d] of workspace [%d] err:%s`,
				attachmentID, wsID, err.Error())
			return err
		}
		if len(ledger) == 0 || ledger[0][`stored_path`] == `` {
			logs.Warning(`template export: knowledge source attachment [%d] of workspace [%d] has no ledger row, skipped`,
				attachmentID, wsID)
			continue
		}
		if byDocument == nil {
			byDocument = make(map[int]string)
			e.sourceAssets[wsID] = byDocument
		}
		byDocument[cast.ToInt(row[`document_id`])] = ledger[0][`stored_path`]
	}
	return nil
}

// exportAsset reads one referenced object and registers it in the index under its package path.
// An object that cannot be read is counted and warned about instead of failing the download: the
// reference to it is then emptied by rewriteAssetReferences, and the copy degrades to a project
// without that image (R-5).
func (e *templateExporter) exportAsset(stored string) error {
	_, key, err := lib_storage.ParseStored(stored)
	if err != nil {
		e.missAsset(stored, `unparseable stored value`)
		return nil
	}
	business, ext, ok := templateStoredNamespace(key, e.adminID)
	if !ok {
		// A foreign tenant's object must never enter a package that ships to every deployment,
		// and a business outside the storage whitelist could escape the upload root on import.
		e.missAsset(stored, `object outside the template tenant or the storage namespaces`)
		return nil
	}
	data, err := lib_storage.M.Read(stored)
	if err != nil {
		e.missAsset(stored, `object cannot be read`)
		return nil
	}
	meta, err := e.ledgerMeta(stored)
	if err != nil {
		return err
	}
	row := templateAssetRow(key, business, ext, data, meta)
	ref := templateAssetEntryPath(row.Sha256, row.Ext)
	e.assets[ref] = row
	e.assetRefs[stored] = ref
	if _, written := e.assetBlobs[ref]; !written {
		e.assetBlobs[ref] = data
	}
	return nil
}

// rewriteAssetReferences replaces every reference to a template storage object with its package
// path, so no exported JSON value points outside the archive. A reference the package could not
// carry is emptied: nothing on a private deployment can resolve the original, and the operator
// is told through missing_asset_count and the export warnings.
func (e *templateExporter) rewriteAssetReferences() {
	for _, row := range e.workspaces {
		row[`icon`] = e.assetReference(row[`icon`])
	}
	for _, wsID := range e.workspaceIDs {
		sources := e.sources[wsID]
		for _, row := range sources[SrcIssueInvites] {
			row[`theme_image`] = e.assetReference(row[`theme_image`])
		}
		for _, row := range sources[SrcIssueItemTemplates] {
			row[`theme_image`] = e.assetReference(row[`theme_image`])
		}
		for _, row := range sources[SrcWorkItems] {
			row[`description`] = e.rewriteContentReferences(row[`description`])
		}
		// File-mode content is empty by construction and never rewritten by the copy, so there
		// is nothing to point at an object.
		for _, row := range sources[SrcKnowledgeDocContents] {
			if row[`content_mode`] == define.KnowledgeContentModeFile {
				continue
			}
			row[`content`] = e.rewriteContentReferences(row[`content`])
		}
		for _, row := range sources[SrcDashboardCards] {
			row[`content`] = e.rewriteContentReferences(row[`content`])
		}
		for _, row := range sources[SrcDashboardAnnouncements] {
			row[`content`] = e.rewriteContentReferences(row[`content`])
		}
	}
}

// assetReference maps one template storage value to its package path; a value the package does
// not carry becomes empty.
func (e *templateExporter) assetReference(stored string) string {
	if stored == `` {
		return ``
	}
	return e.assetRefs[stored]
}

// rewriteContentReferences rewrites the template storage references of one rich text/markdown
// content. The scan is the same one that collected the candidates, so an exported reference is
// always the value being replaced; longer values go first, exactly like the copy side, because
// one stored URL can be a prefix of another.
func (e *templateExporter) rewriteContentReferences(content string) string {
	stored := e.scanner.storedURLsInContent(content)
	if len(stored) == 0 {
		return content
	}
	sort.Slice(stored, func(i, j int) bool { return len(stored[i]) > len(stored[j]) })
	for _, value := range stored {
		content = strings.ReplaceAll(content, value, e.assetReference(value))
	}
	return content
}

// templateAssetRow builds the index row of one exported object. The index key is the content
// hash plus the extension, so identical bytes are stored once and a reference always resolves to
// exactly one content. Ledger metadata wins over the key-derived values when the object is
// registered there, so the destination keeps the file name the uploader saw.
func templateAssetRow(objectKey, business, ext string, data []byte, meta msql.Params) templateAssetIndexRow {
	sha := templateSha256Hex(data)
	if meta != nil && meta[`file_ext`] != `` {
		ext = sanitizeCopiedStoredExt(meta[`file_ext`])
	}
	row := templateAssetIndexRow{
		Business:    business,
		Ext:         ext,
		FileName:    filepath.Base(objectKey),
		FileSize:    int64(len(data)),
		ContentType: templateAssetContentType(ext),
		Sha256:      sha,
	}
	if meta == nil {
		return row
	}
	if name := meta[`file_name`]; name != `` {
		row.FileName = name
	}
	if size := cast.ToInt64(meta[`file_size`]); size > 0 {
		row.FileSize = size
	}
	if contentType := meta[`content_type`]; contentType != `` {
		row.ContentType = contentType
	}
	row.StoredName = cast.ToString(meta[`stored_name`])
	return row
}

// missAsset records a reference the package cannot carry.
func (e *templateExporter) missAsset(stored, reason string) error {
	e.missing++
	logs.Warning(`template export: asset [%s] skipped: %s`, stored, reason)
	return nil
}

// templateStoredNamespace returns the storage namespace and sanitized extension of one object
// key, rejecting keys that are not the template account's or not a known namespace. Both layouts
// are accepted: {business}/{adminID}/{date}/{random}.{ext} and the older
// {adminID}/{business}/{date}/{random}.{ext}, so objects written before the swap still export.
func templateStoredNamespace(key string, adminID int) (string, string, bool) {
	parts := strings.SplitN(key, `/`, 3)
	if len(parts) != 3 {
		return ``, ``, false
	}
	business := ``
	if templateAssetBusinesses[parts[0]] && parts[1] == cast.ToString(adminID) {
		business = parts[0]
	} else if templateAssetBusinesses[parts[1]] && parts[0] == cast.ToString(adminID) {
		business = parts[1]
	} else {
		return ``, ``, false
	}
	return business, sanitizeCopiedStoredExt(filepath.Ext(parts[2])), true
}

// templateAssetContentType mirrors the metadata the copy records for a migrated object.
func templateAssetContentType(ext string) string {
	contentType := mime.TypeByExtension(`.` + ext)
	if contentType == `` {
		return `application/octet-stream`
	}
	return contentType
}

// ledgerMeta returns the template tenant's file ledger row of one stored object, or nil when the
// object was never registered there. The ledger only supplies display metadata (original file
// name, uploader-visible size), so a missing row degrades to key-derived values like today.
func (e *templateExporter) ledgerMeta(stored string) (msql.Params, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT file_name, file_ext, file_size, content_type, stored_name
		FROM `+define.GetEnterpriseFileTable(e.adminID)+`
		WHERE admin_id = $1 AND stored_path = $2
		ORDER BY id ASC LIMIT 1`, nil, e.adminID, stored)
	if err != nil {
		logs.Error(`template export: load ledger metadata of [%s] err:%s`, stored, err.Error())
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// applyKnowledgeSourceAssets fills the source_asset derivation. Only a reference the package
// actually carries becomes a path: an unregistered one then reads like a document without a
// downloadable source, which is what the copy does when the object is missing.
func (e *templateExporter) applyKnowledgeSourceAssets() {
	for wsID, byDocument := range e.sourceAssets {
		for _, row := range e.sources[wsID][SrcKnowledgeDocContents] {
			stored, ok := byDocument[cast.ToInt(row[`document_id`])]
			if !ok {
				continue
			}
			if ref, carried := e.assetRefs[stored]; carried {
				row[TemplateDerivedKnowledgeSourceAsset] = ref
			}
		}
	}
}

// pack writes the archive. Entry names come from the same helpers the reader validates, and
// finish enforces the package size limit; an oversized export fails instead of being trimmed.
func (e *templateExporter) pack() ([]byte, error) {
	now := time.Now().Unix()
	writer := newTemplatePackageWriter(now)
	manifest := templatePackageManifest{
		GeneratedAt:       now,
		TemplateAdminID:   e.adminID,
		WorkspaceIDs:      e.workspaceIDs,
		AssetCount:        len(e.assets),
		MissingAssetCount: e.missing,
	}
	if err := writer.writeJSON(templateManifestEntry, manifest); err != nil {
		return nil, err
	}
	if err := writer.writeJSON(templateWorkspacesEntry, e.workspaces); err != nil {
		return nil, err
	}
	if len(e.poolRows) > 0 {
		if err := writer.writeJSON(templatePoolEntry, e.poolRows); err != nil {
			return nil, err
		}
	}
	for _, wsID := range e.workspaceIDs {
		// Every declared source is written even when empty: the reader judges a project without
		// one of the files corrupt, and an empty array is the honest answer.
		for _, table := range templateTables {
			if err := writer.writeJSON(templateDataEntry(wsID, table.Name), e.sources[wsID][table.Name]); err != nil {
				return nil, err
			}
		}
	}
	if err := writer.writeJSON(templateAssetIndexEntry, e.assets); err != nil {
		return nil, err
	}
	entries := make([]string, 0, len(e.assetBlobs))
	for entry := range e.assetBlobs {
		entries = append(entries, entry)
	}
	sort.Strings(entries)
	for _, entry := range entries {
		if err := writer.writeEntry(entry, e.assetBlobs[entry]); err != nil {
			return nil, err
		}
	}
	return writer.finish()
}

// webhookActionType is the automation action type that carries an outbound target and a secret.
const webhookActionType = `webhook`

// stripAutomationWebhooks removes every webhook action from a rule so a template account's
// outbound targets, signing secrets and mention lists cannot travel in a package that every
// private deployment pulls. The rest of the rule survives: the trigger, the conditions and the
// sibling actions still copy, and the administrator re-adds the notification step in the new
// project. Clearing only the credential fields is not an option, because a webhook with an empty
// URL fails the shape validator and the copy then drops the whole rule. A node or branch whose
// last remaining action was the webhook one cannot satisfy that validator either, so it goes
// too, and a rule left without any node is not exported at all (ok=false). The old
// database-to-database copy carried config_json verbatim, which is the leak this closes.
func stripAutomationWebhooks(configJSON string) (string, bool) {
	config, err := decodeAutomationConfig([]byte(configJSON))
	if err != nil {
		// An undecodable config cannot be proven credential-free, so none of it ships. The copy
		// step skips such a rule the same way it skips one it cannot decode today.
		logs.Warning(`template export: automation config cannot be decoded, rule not exported: %s`, err.Error())
		return ``, false
	}
	nodes := config.Nodes[:0]
	for _, node := range config.Nodes {
		if node.Mode == `direct` {
			if node.Action == nil || node.Action.Type != webhookActionType {
				nodes = append(nodes, node)
			}
			continue
		}
		node.Actions = withoutWebhookActions(node.Actions)
		// A conditional node whose primary branch carried nothing but webhooks cannot lend that
		// branch's place to an else-if: promoting one would evaluate its condition against the
		// node's own modules and only after a primary condition that no longer exists, so the
		// whole node goes.
		if len(node.Actions) == 0 {
			continue
		}
		elseIf := node.ElseIf[:0]
		for _, branch := range node.ElseIf {
			branch.Actions = withoutWebhookActions(branch.Actions)
			if len(branch.Actions) > 0 {
				elseIf = append(elseIf, branch)
			}
		}
		node.ElseIf = elseIf
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		logs.Warning(`template export: automation rule carries nothing but webhooks, rule not exported`)
		return ``, false
	}
	config.Nodes = nodes
	encoded, err := json.Marshal(config)
	if err != nil {
		logs.Warning(`template export: automation config cannot be encoded, rule not exported: %s`, err.Error())
		return ``, false
	}
	return string(encoded), true
}

// withoutWebhookActions compacts one action list in place, dropping the webhook entries.
func withoutWebhookActions(actions []AutomationAction) []AutomationAction {
	kept := actions[:0]
	for _, action := range actions {
		if action.Type != webhookActionType {
			kept = append(kept, action)
		}
	}
	return kept
}
