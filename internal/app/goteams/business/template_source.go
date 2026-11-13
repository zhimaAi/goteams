// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import "goteams/internal/app/goteams/define"

// Template package source keys: one per JSON file inside data/<workspace_id>/ of a template
// package. The value is the entry base name, so it is part of the package format and of the
// copy flow's source API (TemplateSource.Rows).
const (
	SrcWorkspaceStatuses     = `workspace_statuses`
	SrcWorkspacePriorities   = `workspace_priorities`
	SrcRequirementCategories = `requirement_categories`
	SrcWorkspacesSettings    = `workspaces_settings`
	SrcWorkspaceFieldConfigs = `workspace_field_configs`
	SrcWorkspaceCustomFields = `workspace_custom_fields`
	SrcWorkItemAttributes    = `workspace_work_item_attributes`
	SrcWorkspaceApplications = `workspace_applications`
	// SrcIssueInvites carries the project-level invitation theme/guide fields (workspace_issue_invites).
	SrcIssueInvites          = `workspace_issue_invites`
	SrcNavigationSettings    = `workspace_navigation_settings`
	SrcPlanningGroups        = `planning_groups`
	SrcVersions              = `versions`
	SrcWorkItemTemplateLinks = `workspace_work_item_templates`
	// SrcIssueItemTemplates carries the invitation templates' association rows
	// (workspace_issue_item_templates) together with each template's own columns.
	SrcIssueItemTemplates = `workspace_issue_item_templates`
	SrcViews              = `workspace_views`
	SrcAutomationRules    = `automation_rules`
	SrcRoles              = `roles`
	// SrcPlannings carries planning rows of the requirements table (type = planning).
	SrcPlannings             = `requirements_planning`
	SrcWorkItems             = `requirements`
	SrcKnowledgeItems        = `knowledge_items`
	SrcKnowledgeDocContents  = `knowledge_document_contents`
	SrcKnowledgeLinkContents = `knowledge_link_contents`
	// SrcDashboards carries the shared dashboards; creator and timestamps stay behind because the
	// copy attributes them to the operator of the new project.
	SrcDashboards = `workspace_dashboards`
	// SrcDashboardVisibilityRoles carries the per-dashboard role visibility; the copy remaps
	// role_id through the same role id map the roles step produces.
	SrcDashboardVisibilityRoles = `dashboard_visibility_roles`
	SrcDashboardCards           = `dashboard_cards`
	// SrcDashboardAnnouncements carries the one project announcement row (at most one per
	// workspace, like workspace_issue_invites).
	SrcDashboardAnnouncements = `dashboard_announcements`
)

// Derived row keys the exporter appends on top of TemplateTable.Columns. They are computed at
// export time so the copy side needs no extra query for them.
const (
	// TemplateDerivedVersionShared marks an iteration shared with another project ("0"/"1").
	TemplateDerivedVersionShared = `is_shared`
	// TemplateDerivedRolePermissions is the role's permission keys as a JSON array string.
	TemplateDerivedRolePermissions = `permissions`
	// TemplateDerivedIssueItemFields is the invitation template's field schema (folded JOIN).
	TemplateDerivedIssueItemFields = `fields_json`
	// TemplateDerivedKnowledgeSourceAsset is the package path of the object backing a document's
	// source attachment; it is an assets/index.json key, i.e. the value ReadAsset accepts.
	TemplateDerivedKnowledgeSourceAsset = `source_asset`
)

// TemplateTable describes one copyable source of the template flow: the package entry that
// carries it, the physical table, the selected columns and the export order.
//
// Columns is the single source of truth for the source: the exporter selects exactly these
// columns and the package reader validates package entries against this list. It is spelled
// verbatim like the copy step's SELECT, so the copy reads the row shape it always did; the only
// values the exporter rewrites are references to template storage objects, which become package
// paths.
//
// Derived names the extra row keys the exporter appends; DerivedSelect holds the extra SELECT
// expressions producing them. A derivation that needs more than a SQL expression (roles,
// knowledge source assets) leaves DerivedSelect empty and runs in the exporter instead.
type TemplateTable struct {
	Name          string   // package entry base name, one of the Src* keys
	Table         string   // physical table name (define.Table*)
	Columns       []string // export SELECT list, verbatim from the copy flow's SELECT
	Derived       []string // extra row keys appended on top of Columns
	DerivedSelect []string // extra SELECT expressions producing Derived keys (SQL-side derivations)
	From          string   // FROM clause content; defaults to Table
	Where         string   // WHERE clause; defaults to the tenant + workspace predicate
	Args          []string // extra bind arguments after admin id ($1) and workspace id ($2)
	OrderBy       string   // ORDER BY clause; also the array order inside the package
}

// templateTables is the complete source set: 26 entries in the copy flow's dependency order.
// The exporter reads exactly these tables, the package reader validates packages against them,
// and the copy side reads the resulting package entries by the Src* keys.
var templateTables = []TemplateTable{
	{
		Name:    SrcWorkspaceStatuses,
		Table:   define.TableWorkItemStatuses,
		Columns: []string{`id`, `type_key`, `name`, `color`, `sort_order`, `is_default`},
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcWorkspacePriorities,
		Table:   define.TableWorkItemPriorities,
		Columns: []string{`id`, `type_key`, `name`, `color`, `sort_order`, `is_default`},
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcRequirementCategories,
		Table:   define.TableRequirementCategories,
		Columns: []string{`id`, `parent_id`, `name`, `sort_order`},
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:  SrcWorkspacesSettings,
		Table: define.TableWorkspaceSettings,
		Columns: []string{`requirement_initial_status_id`, `defect_initial_status_id`,
			`planning_initial_status_id`, `requirement_final_status_ids`, `defect_final_status_ids`,
			`planning_final_status_ids`, `requirement_completed_status_ids`, `defect_completed_status_ids`, `requirement_template_enabled`, `defect_template_enabled`,
			`requirement_status_transition`, `defect_status_transition`, `requirement_aging_control`},
		// One row per workspace; id keeps the export reproducible.
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcWorkspaceFieldConfigs,
		Table:   define.TableWorkItemFieldConfigs,
		Columns: []string{`type_key`, `field_key`, `is_enabled`, `is_editable`, `is_locked`, `sort_order`},
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcWorkspaceCustomFields,
		Table:   define.TableCustomFields,
		Columns: []string{`id`, `type_key`, `key`, `name`, `field_type`, `is_required`, `sort_order`, `is_enabled`, `config_json`},
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcWorkItemAttributes,
		Table:   define.TableWorkItemAttributes,
		Columns: []string{`type_key`, `name`, `icon_color`, `icon_text`, `description`},
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcWorkspaceApplications,
		Table:   define.TableWorkspaceApplications,
		Columns: []string{`app_key`, `enabled`},
		OrderBy: `ORDER BY app_key ASC`,
	},
	{
		Name:    SrcIssueInvites,
		Table:   define.TableWorkspaceIssueInvites,
		Columns: []string{`theme_image`, `guide_title`, `guide_description`},
		// At most one row per workspace; id keeps the export reproducible.
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcNavigationSettings,
		Table:   define.TableWorkspaceNavigationSettings,
		Columns: []string{`main_nav_order`, `nav_style`, `prevent_member_edit`},
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcPlanningGroups,
		Table:   define.TablePlanningGroups,
		Columns: []string{`id`, `name`},
		Where:   `admin_id = $1 AND workspace_id = $2 AND deleted_at = 0`,
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcVersions,
		Table:   define.TableVersions,
		Columns: []string{`v.id`, `v.name`, `v.start_date`, `v.end_date`, `v.ended_at`},
		Derived: []string{TemplateDerivedVersionShared},
		// The sharing predicate is the copy step's NOT EXISTS in CASE form: a version is shared
		// when another project links it, and shared versions stay behind on the copy side.
		DerivedSelect: []string{`CASE WHEN EXISTS (SELECT 1 FROM ` + define.TableVersionWorkspaces + ` shared
				WHERE shared.admin_id = v.admin_id AND shared.version_id = v.id AND shared.workspace_id <> $2)
			THEN '1' ELSE '0' END AS shared_flag`},
		From:    define.TableVersions + ` v JOIN ` + define.TableVersionWorkspaces + ` vw ON vw.admin_id = v.admin_id AND vw.version_id = v.id`,
		Where:   `v.admin_id = $1 AND vw.workspace_id = $2`,
		OrderBy: `ORDER BY v.id ASC`,
	},
	{
		Name:    SrcWorkItemTemplateLinks,
		Table:   define.TableWorkspaceWorkItemTemplates,
		Columns: []string{`template_id`, `sort_order`},
		OrderBy: `ORDER BY sort_order ASC, id ASC`,
	},
	{
		Name:  SrcIssueItemTemplates,
		Table: define.TableWorkspaceIssueItemTemplates,
		Columns: []string{`a.sort_order`, `a.enabled`, `a.theme_image`, `a.guide_title`, `a.guide_description`,
			`a.guide_enabled`, `a.recall_enabled`, `a.recall_status_id`,
			`t.type_key`, `t.name`, `t.title`, `t.description`, `t.description_mode`},
		Derived: []string{TemplateDerivedIssueItemFields},
		// The copy step joined the template row to read the invitation schema; the package folds
		// that JOIN so the row already carries the invitation template's own columns and
		// fields_json.
		DerivedSelect: []string{`COALESCE(t.fields_json, '') AS fields_json`},
		From: define.TableWorkspaceIssueItemTemplates + ` a
			JOIN ` + define.TableWorkItemTemplates + ` t ON t.admin_id = a.admin_id AND t.id = a.template_id`,
		Where:   `a.admin_id = $1 AND a.workspace_id = $2`,
		OrderBy: `ORDER BY a.sort_order ASC, a.id ASC`,
	},
	{
		Name:  SrcViews,
		Table: define.TableWorkspaceViews,
		// Both copy branches select the same five columns; builtin_key and view_type let the copy
		// side tell the two branches apart (builtin refresh vs. new insert).
		Columns: []string{`type_key`, `builtin_key`, `name`, `filters`, `view_type`},
		// The copy step ran two queries: builtin system views first (ordered by id), then the
		// account's own non-builtin system/personal views (ordered by id). The union keeps that
		// order so the package array matches what the copy used to process.
		From: `(
			SELECT type_key, builtin_key, name, filters, view_type, 0 AS branch, id AS row_id
			FROM ` + define.TableWorkspaceViews + `
			WHERE admin_id = $1 AND workspace_id = $2 AND view_type = '` + ViewTypeSystem + `' AND builtin_key <> ''
			UNION ALL
			SELECT pv.type_key, pv.builtin_key, pv.name, pv.filters, pv.view_type, 1 AS branch, pv.id AS row_id
			FROM ` + define.TableWorkspaceViews + ` pv
			JOIN ` + define.TableUsers + ` u ON u.id = pv.created_by AND u.admin_id = $1
			WHERE pv.admin_id = $1 AND pv.workspace_id = $2 AND pv.builtin_key = ''
				AND pv.view_type IN ('` + ViewTypeSystem + `','` + ViewTypePersonal + `')
		) v`,
		Where:   `1 = 1`,
		OrderBy: `ORDER BY v.branch ASC, v.row_id ASC`,
	},
	{
		Name:    SrcAutomationRules,
		Table:   define.TableAutomationRules,
		Columns: []string{`id`, `name`, `locale`, `config_json`},
		OrderBy: `ORDER BY created_at ASC, id ASC`,
	},
	{
		Name:    SrcRoles,
		Table:   define.TableRoles,
		Columns: []string{`id`, `name`, `description`, `is_system`, `data_view_scope`},
		Where:   `admin_id = $1 AND workspace_id = $2 AND scope = 'project'`,
		Derived: []string{TemplateDerivedRolePermissions},
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcPlannings,
		Table:   define.TableRequirements,
		Columns: []string{`id`, `group_id`, `title`},
		Where: `admin_id = $1 AND workspace_id = $2 AND type = $3
			AND deleted_at = 0 AND is_archived = 0 AND TRIM(title) <> ''`,
		Args:    []string{define.TypeKeyPlanning},
		OrderBy: `ORDER BY group_id ASC, sort_order ASC, id ASC`,
	},
	{
		Name:    SrcWorkItems,
		Table:   define.TableRequirements,
		Columns: []string{`id`, `parent_id`, `type`, `title`, `description`, `description_mode`, `status_id`, `priority`, `version_id`},
		Where:   `admin_id = $1 AND workspace_id = $2 AND type IN ($3, $4) AND deleted_at = 0`,
		Args:    []string{define.TypeKeyRequirement, define.TypeKeyDefect},
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcKnowledgeItems,
		Table:   define.TableKnowledgeItems,
		Columns: []string{`id`, `parent_id`, `item_type`, `title`, `sort_order`},
		Where:   `admin_id = $1 AND workspace_id = $2 AND deleted_at = 0`,
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:    SrcKnowledgeDocContents,
		Table:   define.TableKnowledgeDocumentContents,
		Columns: []string{`document_id`, `content`, `content_mode`, `source_type`, `source_attachment_id`},
		Derived: []string{TemplateDerivedKnowledgeSourceAsset},
		OrderBy: `ORDER BY document_id ASC, id ASC`,
	},
	{
		Name:    SrcKnowledgeLinkContents,
		Table:   define.TableKnowledgeLinkContents,
		Columns: []string{`link_id`, `source_url`},
		OrderBy: `ORDER BY link_id ASC, id ASC`,
	},
	// The dashboard set mirrors CopyDashboardProjectTemplateTx: shared dashboards, role
	// visibility and cards travel; member visibility, extra editors and personal states are
	// user-level state that a template account's members cannot meaningfully export.
	{
		Name:    SrcDashboards,
		Table:   define.TableWorkspaceDashboards,
		Columns: []string{`id`, `name`, `visibility_type`, `sort_order`},
		Where:   `admin_id = $1 AND workspace_id = $2 AND deleted_at = 0`,
		OrderBy: `ORDER BY sort_order ASC, id ASC`,
	},
	{
		Name:    SrcDashboardVisibilityRoles,
		Table:   define.TableDashboardVisibilityRoles,
		Columns: []string{`dashboard_id`, `role_id`},
		OrderBy: `ORDER BY id ASC`,
	},
	{
		Name:  SrcDashboardCards,
		Table: define.TableDashboardCards,
		// config/content travel verbatim; the copy normalises chart and metric configs the way
		// the live copy does (project-scoped ids cannot be reused) and rewrites the asset
		// references the exporter already turned into package paths.
		Columns: []string{`id`, `dashboard_id`, `card_type`, `title`, `config`, `content`,
			`content_mode`, `grid_x`, `grid_y`, `grid_width`, `grid_height`},
		Where: `admin_id = $1 AND workspace_id = $2 AND deleted_at = 0`,
		// Per dashboard in the board order the copy iterates, then the card layout order of
		// loadDashboardCardsForUpdateTx.
		OrderBy: `ORDER BY dashboard_id ASC, grid_y ASC, grid_x ASC, id ASC`,
	},
	{
		Name:    SrcDashboardAnnouncements,
		Table:   define.TableDashboardAnnouncements,
		Columns: []string{`content`, `content_mode`},
		// One row per workspace; id keeps the export reproducible.
		OrderBy: `ORDER BY id ASC`,
	},
}

// templateTableByName returns the source definition of a package entry base name.
func templateTableByName(name string) (TemplateTable, bool) {
	for _, table := range templateTables {
		if table.Name == name {
			return table, true
		}
	}
	return TemplateTable{}, false
}
