// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ErrWorkspaceTemplateInvalid indicates the template workspace is missing, ended,
// or not owned by the configured template account.
var ErrWorkspaceTemplateInvalid = errors.New("invalid template workspace")

// GetTemplateAdminId returns the template account admin_id from configuration.
func GetTemplateAdminId() int {
	return cast.ToInt(define.Config.Template[`workspace_template_id`])
}

// templatePackageUnavailableLog is the fixed keyword ops grep for "the template package cannot
// be read"; it separates packaging failures from a copy that failed midway through a step.
const templatePackageUnavailableLog = `template package unavailable`

// logTemplatePackageUnavailable records a package read failure with its fixed keyword and the
// classified reason, so the degradation is visible in logs without changing the caller's result.
func logTemplatePackageUnavailable(action string, err error) {
	reason := `missing or corrupted`
	if errors.Is(err, ErrTemplatePackageWorkspace) {
		reason = `workspace outside package`
	}
	logs.Error("%s action:%s reason:%s err:%s", templatePackageUnavailableLog, action, reason, err.Error())
}

// ListWorkspaceTemplates returns the active workspaces carried by the template package, in the
// package's own order (manual sort_order then newest id). A package that cannot be read yields
// an empty list instead of an error: the template list endpoint must not fail over packaging.
func ListWorkspaceTemplates() ([]WorkspaceModel, error) {
	pkg, err := LoadTemplatePackage()
	if err != nil {
		logTemplatePackageUnavailable(`list workspace templates`, err)
		return nil, nil
	}
	rows := pkg.Workspaces()
	items := make([]WorkspaceModel, 0, len(rows))
	for _, row := range rows {
		item, err := toWorkspaceModel(row)
		if err != nil {
			logs.Error("convert template workspace row failed, err:%s", err.Error())
			return nil, err
		}
		// The stored URL of the template account resolves only on the deployment that exported
		// it, so a packaged avatar is served by this deployment instead; without bytes the card
		// keeps the letter avatar.
		item.Icon = ``
		if _, carried := pkg.WorkspaceIcon(item.Id); carried {
			item.Icon = templateWorkspaceIconPath(item.Id)
		}
		items = append(items, item)
	}
	return items, nil
}

// templateWorkspaceIconPath is the icon endpoint of one packaged template project.
func templateWorkspaceIconPath(wsID int) string {
	return `/api/workspace-templates/` + strconv.Itoa(wsID) + `/icon`
}

// TemplateWorkspaceIcon returns one template project's packaged avatar bytes and file metadata.
// ok is false for an unknown project and for one whose avatar the package does not carry — the
// list only hands out icon URLs for carried bytes, so every miss is the same 404.
func TemplateWorkspaceIcon(wsID int) ([]byte, TemplateAsset, bool) {
	pkg, err := LoadTemplatePackage()
	if err != nil {
		logTemplatePackageUnavailable(`serve workspace template icon`, err)
		return nil, TemplateAsset{}, false
	}
	ref, carried := pkg.WorkspaceIcon(wsID)
	if !carried {
		return nil, TemplateAsset{}, false
	}
	source, err := pkg.NewSource(wsID)
	if err != nil {
		return nil, TemplateAsset{}, false
	}
	return source.ReadAsset(ref)
}

// TemplateConfigCopier orchestrates copying a template workspace's configuration to a new workspace.
//
// Every source row comes from the offline template package through src; the copier never reads the
// template account's database or storage. Timestamps follow two contracts that must not be mixed:
// record columns (created_at, updated_at, enabled_at, created_by, updated_by, creator_id, uploader
// and creator snapshots) always take c.now / c.userId, while state columns carried by the copied
// configuration itself (versions.ended_at / start_date / end_date, sort_order values,
// workspace_statuses.is_default) keep the package value. automation_rules.created_at inside the
// package is only an ordering input and is still written as c.now.
type TemplateConfigCopier struct {
	adminId            int     // current user's admin_id (destination)
	userId             int     // current user's user_id (records configuration actions)
	templateAdminId    int     // package manifest admin_id, used only to match asset URLs inside copied content
	templateWsId       int     // template workspace ID carried by the package (source)
	newWsId            int     // newly created workspace ID (destination)
	now                int     // unix timestamp
	tx                 *sql.Tx // shared transaction handle; every copy step must execute on it
	src                *TemplateSource
	issueEnabled       bool   // whether the destination must initialize the issue-invitation application
	uploaderName       string // cached operator display name recorded on copied file ledger rows
	copiedStorageDelta int64  // total bytes of ledger rows inserted by this copy, applied to the tenant storage usage cache after commit
	// copiedAssets deduplicates invitation theme image uploads across the copy steps.
	copiedAssets map[string]copiedAssetInfo
	mappings     templateIDMappings
}

type templateIDMappings struct {
	statuses       map[int]int // old workspace_statuses.id -> new workspace_statuses.id
	priorities     map[int]int // old workspace_priorities.id -> new workspace_priorities.id
	categories     map[int]int // old requirement_categories.id -> new requirement_categories.id
	versions       map[int]int // old versions.id -> new versions.id
	planningGroups map[int]int // old planning_groups.id -> new planning_groups.id
	customFields   map[int]int // old workspace_custom_fields.id -> new workspace_custom_fields.id
	roles          map[int]int // old roles.id -> new roles.id (member role included, admin role excluded)
}

// CopyTemplateFromWorkspace is the entry point called after creating a workspace with a template.
// Every verification and copy step runs inside one transaction: any failure rolls the whole
// copy back, leaving the new workspace with its seed configuration only, and the error is
// returned to the caller.
func CopyTemplateFromWorkspace(params define.CopyWorkspaceTemplateModel) error {
	if params.AdminID <= 0 || params.OperatorID <= 0 || params.TemplateWorkspaceID <= 0 || params.NewWorkspaceID <= 0 {
		return ErrWorkspaceTemplateInvalid
	}
	// Same source and destination would let the delete-first steps wipe the template
	// workspace itself; reject before touching any data.
	if params.TemplateWorkspaceID == params.NewWorkspaceID {
		return ErrWorkspaceTemplateInvalid
	}
	c := &TemplateConfigCopier{
		adminId:      params.AdminID,
		userId:       params.OperatorID,
		templateWsId: params.TemplateWorkspaceID,
		newWsId:      params.NewWorkspaceID,
		now:          tool.Time2Int(),
		copiedAssets: make(map[string]copiedAssetInfo),
		mappings: templateIDMappings{
			statuses:       make(map[int]int),
			priorities:     make(map[int]int),
			categories:     make(map[int]int),
			versions:       make(map[int]int),
			planningGroups: make(map[int]int),
			customFields:   make(map[int]int),
			roles:          make(map[int]int),
		},
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin copy template [%d] tx failed: %s", c.templateWsId, err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := c.run(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		logs.Error("commit copy template [%d] tx failed: %s", c.templateWsId, err.Error())
		return err
	}
	// The copy committed: fold the copied files' size into the tenant storage usage cache.
	// Adjusting before commit would overstate usage if a later step rolled the tx back.
	if c.copiedStorageDelta > 0 {
		adjustStorageUsedCache(c.adminId, c.copiedStorageDelta)
	}
	// Copied items carry the template's status and iteration, so the copied iterations'
	// counters (inserted as 0) are recounted from the committed rows. The recount reads
	// settings and work items on its own connection, which is why it cannot run in the tx.
	if err := RecalcWorkspaceVersionCounts(c.adminId, c.newWsId); err != nil {
		logs.Error("copy template [%d] recalc new workspace iteration counts failed: %s", c.templateWsId, err.Error())
	}
	return nil
}

// verifyTargetWorkspace ensures the copy destination is an existing workspace owned by
// the caller, so the delete-first copy steps can never touch a project belonging to
// another tenant or a workspace that does not exist.
func (c *TemplateConfigCopier) verifyTargetWorkspace() error {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableWorkspaces+`
		WHERE admin_id = $1 AND id = $2`, c.tx, c.adminId, c.newWsId)
	if err != nil {
		logs.Error("find target workspace err:%s", err.Error())
		return err
	}
	if len(rows) == 0 {
		return ErrWorkspaceTemplateInvalid
	}
	return nil
}

// verifyTemplateSource resolves the template project inside the offline package. The package
// is the only source of template rows, so an id it does not carry is as invalid as a project
// that never belonged to the template account; the whitelist itself is enforced by NewSource,
// which keeps the check out of every caller's hands.
func (c *TemplateConfigCopier) verifyTemplateSource() error {
	pkg, err := LoadTemplatePackage()
	if err != nil {
		logTemplatePackageUnavailable(`copy template`, err)
		return err
	}
	src, err := pkg.NewSource(c.templateWsId)
	if err != nil {
		logTemplatePackageUnavailable(`copy template`, err)
		return ErrWorkspaceTemplateInvalid
	}
	// Content of the copied rows references the account the package was exported from.
	c.templateAdminId = pkg.TemplateAdminID()
	c.src = src
	return nil
}

// sourceRows reads one package source of the template project. A missing source is an error
// rather than an empty set: silently copying an empty project would hide a broken package.
func (c *TemplateConfigCopier) sourceRows(key string) ([]msql.Params, error) {
	rows, err := c.src.Rows(key)
	if err != nil {
		logs.Error("copy template [%d] package source %s err:%s", c.templateWsId, key, err.Error())
		return nil, err
	}
	return rows, nil
}

// run executes the verifications and every copy step on the shared transaction; the first
// error aborts the remaining steps and the caller's deferred rollback undoes all copy writes.
func (c *TemplateConfigCopier) run(tx *sql.Tx) error {
	c.tx = tx
	if err := c.verifyTemplateSource(); err != nil {
		return err
	}
	if err := c.verifyTargetWorkspace(); err != nil {
		return err
	}
	steps := []struct {
		name string
		fn   func() error
	}{
		{`workspace_statuses`, c.copyStatuses},
		{`workspace_priorities`, c.copyPriorities},
		{`requirement_categories`, c.copyCategories},
		//{`workspace_status_transitions`, c.copyStatusTransitions},
		{`workspaces_settings`, c.copySettings},
		{`workspace_field_configs`, c.copyFieldConfigs},
		{`workspace_custom_fields`, c.copyCustomFields},
		{`workspace_work_item_attributes`, c.copyWorkItemAttributes},
		{`workspace_applications`, c.copyApplications},
		// Navigation entries are application keys, so this runs after the applications copy.
		{`workspace_navigation_settings`, c.copyNavigationSettings},
		{`planning_groups`, c.copyPlanningGroups},
		{`versions`, c.copyVersions},
		// Templates run after versions: a template field's default status/priority/version/category
		// is stored as an option id, so the remap needs every option mapping already built.
		{`workspace_work_item_templates`, c.copyWorkItemTemplates},
		{`workspace_issue_item_templates`, c.copyIssueInviteTemplates},
		// Views run after categories and versions so copied view filters can remap those ids.
		{`workspace_views`, c.copyViews},
		// Automation rules carry status/priority/iteration row ids, so they copy after those
		// mappings are complete.
		{`automation_rules`, c.copyAutomationRules},
		{`roles`, c.copyRoles},
		// Dashboards run after roles: a role-visible board's dashboard_visibility_roles rows
		// are remapped through the role id map the roles step just built.
		{`dashboards`, c.copyDashboards},
		// Data steps run after the config steps: copied work items need the destination's
		// initial status / default priority, which earlier steps already wrote into this tx.
		{`plannings`, c.copyPlannings},
		{`requirements`, c.copyWorkItems},
		{`knowledge_base`, c.copyKnowledgeItems},
	}
	for _, step := range steps {
		if err := step.fn(); err != nil {
			logs.Error("copy template [%d] step %s failed: %s", c.templateWsId, step.name, err.Error())
			return fmt.Errorf("copy %s: %w", step.name, err)
		}
		logs.Info("copy template [%d] step [%s] done", c.templateWsId, step.name)
	}
	return nil
}

// ======================== workspace_statuses ========================

func (c *TemplateConfigCopier) copyStatuses() error {
	rows, err := c.sourceRows(SrcWorkspaceStatuses)
	if err != nil {
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableWorkItemStatuses+`
		WHERE admin_id = $1 AND workspace_id = $2`, c.tx, c.adminId, c.newWsId); err != nil {
		logs.Error("delete new workspace statuses err:%s", err.Error())
		return err
	}
	for _, row := range rows {
		ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkItemStatuses+`
			(admin_id, workspace_id, type_key, name, color, sort_order, is_default, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, c.tx,
			c.adminId, c.newWsId, row[`type_key`], row[`name`], row[`color`],
			row[`sort_order`], row[`is_default`], c.now, c.now)
		if err != nil {
			return err
		}
		c.mappings.statuses[cast.ToInt(row[`id`])] = cast.ToInt(ids[0][`id`])
	}
	return nil
}

// ======================== workspace_priorities ========================

func (c *TemplateConfigCopier) copyPriorities() error {
	rows, err := c.sourceRows(SrcWorkspacePriorities)
	if err != nil {
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableWorkItemPriorities+`
		WHERE admin_id = $1 AND workspace_id = $2`, c.tx, c.adminId, c.newWsId); err != nil {
		logs.Error("delete new workspace priorities err:%s", err.Error())
		return err
	}
	for _, row := range rows {
		ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkItemPriorities+`
			(admin_id, workspace_id, type_key, name, color, sort_order, is_default, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, c.tx,
			c.adminId, c.newWsId, row[`type_key`], row[`name`], row[`color`],
			row[`sort_order`], row[`is_default`], c.now, c.now)
		if err != nil {
			return err
		}
		c.mappings.priorities[cast.ToInt(row[`id`])] = cast.ToInt(ids[0][`id`])
	}
	return nil
}

// ======================== workspaces_settings ========================

func (c *TemplateConfigCopier) copySettings() error {
	rows, err := c.sourceRows(SrcWorkspacesSettings)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	row := rows[0]
	mappedInitial := func(key string) string {
		return cast.ToString(c.mappings.statuses[cast.ToInt(row[key])])
	}
	mappedFinal := func(key string) string {
		ids := textToIntList(row[key])
		mapped := make([]int, 0, len(ids))
		for _, id := range ids {
			if m := c.mappings.statuses[id]; m > 0 {
				mapped = append(mapped, m)
			}
		}
		return joinIntSlice(mapped)
	}
	// 离线模板可能早于状态分类功能导出，缺少完成字段时按历史迁移规则拆分；显式空集合必须保留。
	for _, typ := range []string{define.TypeKeyRequirement, define.TypeKeyDefect} {
		field := typ + "_completed_status_ids"
		if _, exists := row[field]; exists {
			continue
		}
		statuses, err := c.sourceRows(SrcWorkspaceStatuses)
		if err != nil {
			return err
		}
		completed := []int{}
		terminal := toIntSlice(row[typ+"_final_status_ids"])
		for _, status := range statuses {
			id := cast.ToInt(status["id"])
			if status["type_key"] == typ && status["is_default"] == "1" &&
				(status["name"] == lib_i18n.WrapKey("workitem_status_done") || typ == define.TypeKeyDefect && status["name"] == lib_i18n.WrapKey("workitem_status_resolved")) && intSliceContains(terminal, id) {
				completed = append(completed, id)
			}
		}
		row[field] = joinIntSlice(completed)
	}
	// Aging control keeps one timing rule per status id, so it goes through the same remap as
	// the initial/final status columns.
	agingControl, err := c.mappedAgingControl(row[`requirement_aging_control`])
	if err != nil {
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceSettings+` SET
		requirement_initial_status_id = $1, defect_initial_status_id = $2, planning_initial_status_id = $3,
		requirement_final_status_ids = $4, defect_final_status_ids = $5, planning_final_status_ids = $6,
		requirement_template_enabled = $7, defect_template_enabled = $8,
		requirement_status_transition = $9, defect_status_transition = $10,
		requirement_aging_control = $11, updated_at = $12,
		requirement_completed_status_ids = $15, defect_completed_status_ids = $16
		WHERE admin_id = $13 AND workspace_id = $14`, c.tx,
		mappedInitial(`requirement_initial_status_id`), mappedInitial(`defect_initial_status_id`),
		mappedInitial(`planning_initial_status_id`),
		mappedFinal(`requirement_final_status_ids`), mappedFinal(`defect_final_status_ids`),
		mappedFinal(`planning_final_status_ids`),
		row[`requirement_template_enabled`], row[`defect_template_enabled`],
		row[`requirement_status_transition`], row[`defect_status_transition`],
		agingControl, c.now, c.adminId, c.newWsId, mappedFinal(`requirement_completed_status_ids`), mappedFinal(`defect_completed_status_ids`)); err != nil {
		logs.Error("update workspace settings err:%s", err.Error())
		return err
	}
	return nil
}

// mappedAgingControl rewrites the per-status rules of the copied aging control: every rule's
// status_id is a template workspace status id and is remapped like any other copied status
// reference. A rule whose status was not copied is dropped — it can never match a status of the
// new project, and keeping a foreign id would only make the rule look configured. The value is
// re-encoded through the parse/encode pair, so the column stays in the shape the reader expects.
func (c *TemplateConfigCopier) mappedAgingControl(raw string) (string, error) {
	form := parseAgingControlForm(raw)
	rules := make([]AgingStatusRule, 0, len(form.StatusRules))
	for _, rule := range form.StatusRules {
		newId, ok := c.mappings.statuses[rule.StatusID]
		if !ok || newId <= 0 {
			continue
		}
		rule.StatusID = newId
		rules = append(rules, rule)
	}
	form.StatusRules = rules
	return encodeAgingControlForm(form)
}

// ======================== workspace_field_configs ========================

func (c *TemplateConfigCopier) copyFieldConfigs() error {
	rows, err := c.sourceRows(SrcWorkspaceFieldConfigs)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkItemFieldConfigs+`
			(admin_id, workspace_id, type_key, field_key, is_enabled, is_editable, is_locked, sort_order, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (admin_id, workspace_id, type_key, field_key) DO UPDATE SET
				is_enabled = EXCLUDED.is_enabled,
				is_editable = EXCLUDED.is_editable,
				is_locked = EXCLUDED.is_locked,
				sort_order = EXCLUDED.sort_order,
				updated_at = EXCLUDED.updated_at`, c.tx,
			c.adminId, c.newWsId, row[`type_key`], row[`field_key`], row[`is_enabled`],
			row[`is_editable`], row[`is_locked`], row[`sort_order`], c.now, c.now); err != nil {
			return err
		}
	}
	return nil
}

// ======================== workspace_custom_fields ========================

func (c *TemplateConfigCopier) copyCustomFields() error {
	rows, err := c.sourceRows(SrcWorkspaceCustomFields)
	if err != nil {
		return err
	}
	for _, row := range rows {
		ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableCustomFields+`
			(admin_id, workspace_id, type_key, key, name, field_type, is_required, sort_order, is_enabled, config_json, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`, c.tx,
			c.adminId, c.newWsId, row[`type_key`], row[`key`], row[`name`], row[`field_type`],
			row[`is_required`], row[`sort_order`], row[`is_enabled`], row[`config_json`], c.now, c.now)
		if err != nil {
			return err
		}
		c.mappings.customFields[cast.ToInt(row[`id`])] = cast.ToInt(ids[0][`id`])
	}
	return nil
}

// ======================== workspace_work_item_attributes ========================

// copyWorkItemAttributes copies the work item display attributes (requirement/defect
// name, icon, category description) so the new workspace keeps the template's
// per-type presentation. Attribute ids are never referenced elsewhere, so no id
// mapping is needed; the upsert keeps the step idempotent for repeated runs.
func (c *TemplateConfigCopier) copyWorkItemAttributes() error {
	rows, err := c.sourceRows(SrcWorkItemAttributes)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkItemAttributes+`
			(admin_id, workspace_id, type_key, name, icon_color, icon_text, description, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (admin_id, workspace_id, type_key) DO UPDATE SET
				name = EXCLUDED.name,
				icon_color = EXCLUDED.icon_color,
				icon_text = EXCLUDED.icon_text,
				description = EXCLUDED.description,
				updated_at = EXCLUDED.updated_at`, c.tx,
			c.adminId, c.newWsId, row[`type_key`], row[`name`], row[`icon_color`],
			row[`icon_text`], row[`description`], c.now, c.now); err != nil {
			return err
		}
	}
	return nil
}

// ======================== workspace_applications ========================

func (c *TemplateConfigCopier) copyApplications() error {
	rows, err := c.sourceRows(SrcWorkspaceApplications)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row[`app_key`] == define.AppKeyIssue && cast.ToInt(row[`enabled`]) == 1 {
			c.issueEnabled = true
			if _, err := ensureIssueInviteConfigTx(c.tx, c.adminId, c.newWsId, c.userId, int64(c.now)); err != nil {
				return err
			}
			// Copy the workspace-level invitation theme/guide fields from the template workspace.
			inviteRows, qErr := c.sourceRows(SrcIssueInvites)
			if qErr != nil {
				return qErr
			}
			if len(inviteRows) > 0 {
				ir := inviteRows[0]
				themeImage, tErr := c.copyCopiedThemeImage(ir[`theme_image`])
				if tErr != nil {
					return tErr
				}
				if _, upErr := msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceIssueInvites+` SET
					theme_image = $3, guide_title = $4, guide_description = $5, updated_at = $6
					WHERE admin_id = $1 AND workspace_id = $2`, c.tx,
					c.adminId, c.newWsId, themeImage, ir[`guide_title`], ir[`guide_description`], c.now); upErr != nil {
					logs.Error(`update workspace issue invite config fields err:%s`, upErr.Error())
					return upErr
				}
			}
		}
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceApplications+`
			(admin_id, workspace_id, app_key, enabled, enabled_by, enabled_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (admin_id, workspace_id, app_key) DO UPDATE SET
				enabled = EXCLUDED.enabled,
				enabled_by = EXCLUDED.enabled_by,
				enabled_at = EXCLUDED.enabled_at,
				updated_at = EXCLUDED.updated_at`, c.tx,
			c.adminId, c.newWsId, row[`app_key`], row[`enabled`], c.userId, c.now, c.now); err != nil {
			return err
		}
	}
	return nil
}

// ======================== workspace_navigation_settings ========================

// copyNavigationSettings copies the template workspace's project navigation policy (main nav
// order, style and the member-edit lock). The order holds application keys rather than row ids,
// so nothing needs remapping. The per-member snapshots written by "apply to all members"
// (user_workspace_navigation_settings) are member data and stay behind.
func (c *TemplateConfigCopier) copyNavigationSettings() error {
	rows, err := c.sourceRows(SrcNavigationSettings)
	if err != nil {
		return err
	}
	// The template account never touched the navigation menu: the new workspace keeps its defaults.
	if len(rows) == 0 {
		return nil
	}
	row := rows[0]
	if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceNavigationSettings+`
		(admin_id, workspace_id, main_nav_order, nav_style, prevent_member_edit, updated_by, updated_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$7)
		ON CONFLICT (admin_id, workspace_id) DO UPDATE SET
			main_nav_order = EXCLUDED.main_nav_order,
			nav_style = EXCLUDED.nav_style,
			prevent_member_edit = EXCLUDED.prevent_member_edit,
			updated_by = EXCLUDED.updated_by,
			updated_at = EXCLUDED.updated_at`, c.tx,
		c.adminId, c.newWsId, row[`main_nav_order`], row[`nav_style`],
		row[`prevent_member_edit`], c.userId, c.now); err != nil {
		logs.Error("copy workspace navigation settings err:%s", err.Error())
		return err
	}
	return nil
}

// ======================== workspace_views ========================

// copyViews copies views from the template workspace to the destination:
//   - builtin system views are refreshed in place (UPDATE by type_key + builtin_key);
//   - user-defined views (custom system views visible to every member and personal
//     views) are inserted with created_by mapped to the current operator.
func (c *TemplateConfigCopier) copyViews() error {
	rows, err := c.sourceRows(SrcViews)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row[`builtin_key`] == `` {
			continue
		}
		// 拆分状态后导出的旧模板可能把内置完成视图缩小为仅完成。
		if row["builtin_key"] == define.ViewBuiltinFinished {
			row["filters"] = strings.ReplaceAll(row["filters"], strconv.Quote(define.ViewValueAllCompleted), strconv.Quote(define.ViewValueAllEnd))
		}
		// A missing destination view updates 0 rows, which matches the previous skip behaviour.
		if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceViews+` SET
			name = $3, filters = $4, updated_at = $5
			WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $6 AND builtin_key = $7`, c.tx,
			c.adminId, c.newWsId, row[`name`], c.remapViewFilterIDs(row[`filters`]), c.now,
			row[`type_key`], row[`builtin_key`]); err != nil {
			return err
		}
	}
	// User-defined views (builtin_key is empty) created by users of the template account,
	// each keeping its own view type. The package rows are already filtered to that account.
	for _, row := range rows {
		if row[`builtin_key`] != `` {
			continue
		}
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceViews+`
			(admin_id, workspace_id, type_key, name, view_type, builtin_key, filters, created_by, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,'',$6,$7,$8,$9)`, c.tx,
			c.adminId, c.newWsId, row[`type_key`], row[`name`], row[`view_type`],
			c.remapViewFilterIDs(row[`filters`]), c.userId, c.now, c.now); err != nil {
			return err
		}
	}
	return nil
}

// remapViewFilterIDs rewrites stored view filters so status/priority/category/version conditions
// reference the new workspace's ids instead of the template workspace's. Sentinel values and
// non-id fields are left untouched; ids absent from the mapping keep their old value so the view
// keeps resolving to an empty result, matching the behaviour on the template side.
func (c *TemplateConfigCopier) remapViewFilterIDs(filters string) string {
	if strings.TrimSpace(filters) == `` {
		return filters
	}
	var root any
	if err := json.Unmarshal([]byte(filters), &root); err != nil {
		logs.Error("parse template view filters failed, err:%s", err.Error())
		return filters
	}
	if _, ok := root.(map[string]any); !ok {
		if _, ok := root.([]any); !ok {
			return filters
		}
	}
	c.remapFilterNode(root)
	out, err := json.Marshal(root)
	if err != nil {
		logs.Error("marshal remapped template view filters failed, err:%s", err.Error())
		return filters
	}
	return string(out)
}

func (c *TemplateConfigCopier) remapFilterNode(node any) {
	switch value := node.(type) {
	case map[string]any:
		var mapping map[int]int
		switch value[`field`] {
		case define.ViewFilterFieldStatus:
			mapping = c.mappings.statuses
		case define.ViewFilterFieldPriority:
			mapping = c.mappings.priorities
		case define.ViewFilterFieldCategory:
			mapping = c.mappings.categories
		case define.ViewFilterFieldVersion:
			mapping = c.mappings.versions
		}
		if len(mapping) > 0 {
			if rawValues, ok := value[`values`].([]any); ok {
				for i, raw := range rawValues {
					if mapped := remapNumericID(raw, mapping); mapped != `` {
						rawValues[i] = mapped
					}
				}
			}
		}
		for _, child := range value {
			c.remapFilterNode(child)
		}
	case []any:
		for _, child := range value {
			c.remapFilterNode(child)
		}
	}
}

// remapNumericID returns the mapped id string when raw is a plain positive numeric id of the
// template workspace; any other value returns empty.
func remapNumericID(raw any, mapping map[int]int) string {
	var text string
	switch v := raw.(type) {
	case string:
		text = v
	case float64:
		text = strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ``
	}
	text = strings.TrimSpace(text)
	if text == `` || strings.HasPrefix(text, `__`) {
		return ``
	}
	// Strict integer parsing keeps malformed values (e.g. decimals) untouched.
	oldId, err := strconv.Atoi(text)
	if err != nil || oldId <= 0 {
		return ``
	}
	newId, ok := mapping[oldId]
	if !ok || newId <= 0 {
		return ``
	}
	return strconv.Itoa(newId)
}

// ======================== planning_groups ========================

func (c *TemplateConfigCopier) copyPlanningGroups() error {
	rows, err := c.sourceRows(SrcPlanningGroups)
	if err != nil {
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TablePlanningGroups+`
		WHERE admin_id = $1 AND workspace_id = $2`, c.tx, c.adminId, c.newWsId); err != nil {
		logs.Error("delete new workspace planning groups err:%s", err.Error())
		return err
	}
	for _, row := range rows {
		ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TablePlanningGroups+`
			(admin_id, workspace_id, name, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`, c.tx,
			c.adminId, c.newWsId, row[`name`], c.now, c.now)
		if err != nil {
			return err
		}
		// Copied planning rows are attached to their group by this mapping.
		c.mappings.planningGroups[cast.ToInt(row[`id`])] = cast.ToInt(ids[0][`id`])
	}
	return nil
}

// ======================== workspace_work_item_templates ========================

func (c *TemplateConfigCopier) copyWorkItemTemplates() error {
	rows, err := c.sourceRows(SrcWorkItemTemplateLinks)
	if err != nil {
		return err
	}
	// Pool templates are isolated per admin_id, so links pointing at the template
	// account's pool would never resolve for the destination tenant. Duplicate each
	// referenced pool template into the destination tenant's pool and remap the ids.
	templateMapping := make(map[int]int)
	for _, row := range rows {
		oldTemplateId := cast.ToInt(row[`template_id`])
		newTemplateId := templateMapping[oldTemplateId]
		if newTemplateId == 0 {
			copiedId, err := c.copyPoolTemplate(oldTemplateId)
			if err != nil {
				return err
			}
			if copiedId == 0 {
				// The source pool template no longer exists: skip the dangling link.
				logs.Error("copy template [%d] skipped missing pool template [%d]", c.templateWsId, oldTemplateId)
				continue
			}
			templateMapping[oldTemplateId] = copiedId
			newTemplateId = copiedId
		}
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceWorkItemTemplates+`
			(admin_id, workspace_id, template_id, sort_order, created_at)
			VALUES ($1,$2,$3,$4,$5)`, c.tx,
			c.adminId, c.newWsId, newTemplateId, row[`sort_order`], c.now); err != nil {
			return err
		}
	}
	return nil
}

// copyPoolTemplate duplicates a tenant-level pool template carried by the package into the
// destination tenant's pool so copied workspace links resolve locally. It returns 0 when the
// package does not carry the referenced pool template.
func (c *TemplateConfigCopier) copyPoolTemplate(templateId int) (int, error) {
	row, ok := c.src.PoolTemplate(templateId)
	if !ok {
		return 0, nil
	}
	fieldsJSON, err := c.remapCopiedTemplateFields(row[`fields_json`])
	if err != nil {
		return 0, err
	}
	ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkItemTemplates+`
		(admin_id, source_workspace_id, type_key, name, title, description, description_mode, fields_json, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`, c.tx,
		c.adminId, c.newWsId, row[`type_key`], row[`name`], row[`title`], row[`description`],
		row[`description_mode`], fieldsJSON, c.userId, c.now, c.now)
	if err != nil {
		return 0, err
	}
	return cast.ToInt(ids[0][`id`]), nil
}

// remapCopiedTemplateFields rewrites a copied template's stored fields_json from the template
// workspace's ids to the copied workspace's ids: custom field keys, the status/priority/
// version/category option ids kept as field defaults and as linkage conditions, and the linkage
// target keys. The copy is written as a self-contained template of the new workspace
// (source_workspace_id points at it), so the read-time cross-workspace resolution never runs on
// it and a foreign id left here would be consumed as-is: custom fields outside the copied set
// are dropped and unresolvable option ids are cleared, matching the read-time skip/null rules.
// Option-name snapshots go away with a rewritten default because the copied rows are what the
// new workspace resolves against, and copied version names may even have been de-duplicated.
func (c *TemplateConfigCopier) remapCopiedTemplateFields(fieldsJSON string) (string, error) {
	if strings.TrimSpace(fieldsJSON) == `` {
		return fieldsJSON, nil
	}
	fields, err := decodeTemplateFields(fieldsJSON)
	if err != nil {
		return ``, err
	}
	// Custom field keys are also used as linkage targets, so the key map has to cover the
	// whole list before any linkage is rewritten.
	fieldKeys := make(map[string]string, len(fields))
	kept := make([]TemplateFieldModel, 0, len(fields))
	for _, field := range fields {
		oldKey := field.FieldKey
		customId := templateCustomFieldID(oldKey)
		if customId == 0 {
			fieldKeys[oldKey] = oldKey
			kept = append(kept, field)
			continue
		}
		newId, mapped := c.mappings.customFields[customId]
		if !mapped {
			logs.Info("copy template [%d] drops template field [%d] whose custom field is outside the copied set", c.templateWsId, customId)
			continue
		}
		field.FieldKey = fmt.Sprintf(`custom:%d`, newId)
		fieldKeys[oldKey] = field.FieldKey
		kept = append(kept, field)
	}
	out := make([]TemplateFieldModel, 0, len(kept))
	for _, field := range kept {
		field.DefaultValue, field.DefaultNames = c.remapCopiedOptionRef(field.FieldKey, field.DefaultValue, field.DefaultNames)
		linkages := make([]TemplateLinkageModel, 0, len(field.Linkages))
		for _, linkage := range field.Linkages {
			value, names := c.remapCopiedOptionRef(field.FieldKey, linkage.Value, linkage.ValueNames)
			if strings.TrimSpace(string(value)) == `null` {
				continue
			}
			targets := make([]string, 0, len(linkage.FieldKeys))
			for _, key := range linkage.FieldKeys {
				if target, mapped := fieldKeys[key]; mapped {
					targets = append(targets, target)
				}
			}
			if len(targets) == 0 {
				continue
			}
			linkage.Value, linkage.ValueNames, linkage.FieldKeys = value, names, targets
			linkages = append(linkages, linkage)
		}
		field.Linkages = linkages
		out = append(out, field)
	}
	return encodeTemplateFields(out)
}

// remapCopiedOptionRef rewrites one option id a template field stores for a builtin
// status/priority/version/category default or linkage condition; every other field type is
// returned untouched. An id the mappings cannot resolve becomes null instead of a foreign id.
func (c *TemplateConfigCopier) remapCopiedOptionRef(fieldKey string, value json.RawMessage, names []string) (json.RawMessage, []string) {
	var mapping map[int]int
	switch fieldKey {
	case `assignee`:
		// Assignee defaults are user ids of the template account's members. They are no
		// members of the new project, and the read-time member filter empties them anyway.
		return json.RawMessage(`[]`), nil
	case `status`:
		mapping = c.mappings.statuses
	case `priority`:
		mapping = c.mappings.priorities
	case `version`:
		mapping = c.mappings.versions
	case `category`:
		mapping = c.mappings.categories
	default:
		return value, names
	}
	var id int
	if len(value) == 0 || json.Unmarshal(value, &id) != nil || id <= 0 {
		return value, names
	}
	newId := mapping[id]
	if newId <= 0 {
		return json.RawMessage(`null`), nil
	}
	encoded, _ := json.Marshal(newId)
	return encoded, nil
}

// copyIssueInviteTemplates recreates the template project's invitation templates in the new
// project. An invitation template is an entity of its own, so its whole row travels in the
// package; only the custom field ids inside fields_json need remapping, which the pool-template
// remap already does.
func (c *TemplateConfigCopier) copyIssueInviteTemplates() error {
	if !c.issueEnabled {
		return nil
	}
	rows, err := c.sourceRows(SrcIssueItemTemplates)
	if err != nil {
		return err
	}
	for _, row := range rows {
		themeImage, err := c.copyCopiedThemeImage(row[`theme_image`])
		if err != nil {
			return err
		}
		fieldsJSON, err := c.remapCopiedTemplateFields(row[TemplateDerivedIssueItemFields])
		if err != nil {
			return err
		}
		// 撤回状态是项目自己的状态 id，要跟着状态映射换成本项目的；映射不到（状态被删了）就关掉撤回，
		// 不留一个指向空状态的开关。
		recallEnabled := cast.ToInt(row[`recall_enabled`])
		recallStatusID := c.mappings.statuses[cast.ToInt(row[`recall_status_id`])]
		if recallEnabled == 1 && recallStatusID <= 0 {
			recallEnabled = 0
			recallStatusID = 0
		}
		// 模板级邀请文案开关跟着模板走；老导出包里没有这一列时按开启处理，维持旧行为
		guideEnabled := 1
		if raw, exists := row[`guide_enabled`]; exists && cast.ToInt(raw) == 0 {
			guideEnabled = 0
		}
		if _, err = insertIssueInviteTemplateTx(issueInviteTemplateInsertModel{
			Tx: c.tx, AdminID: c.adminId, WorkspaceID: c.newWsId,
			TypeKey: row[`type_key`], Name: row[`name`], DescriptionMode: row[`description_mode`],
			FieldsJSON: fieldsJSON, ThemeImage: themeImage,
			GuideTitle: row[`guide_title`], GuideDescription: row[`guide_description`],
			GuideEnabled: guideEnabled,
			Enabled:      cast.ToInt(row[`enabled`]), SortOrder: cast.ToInt(row[`sort_order`]),
			RecallEnabled: recallEnabled, RecallStatusID: recallStatusID,
			OperatorID: c.userId, Now: int64(c.now),
		}); err != nil {
			return err
		}
	}
	return nil
}

// ======================== versions ========================

func (c *TemplateConfigCopier) copyVersions() error {
	// Cross-project iterations (shared with another workspace) stay behind: the new project is
	// not part of that sharing, so copying one would only duplicate it as an orphan. The
	// package precomputes that decision as is_shared, with the same predicate the old
	// NOT EXISTS applied.
	all, err := c.sourceRows(SrcVersions)
	if err != nil {
		return err
	}
	rows := make([]msql.Params, 0, len(all))
	for _, row := range all {
		if row[TemplateDerivedVersionShared] == `1` {
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	// Defensively clear existing destination rows so the copy stays idempotent.
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableVersionWorkspaces+`
		WHERE admin_id = $1 AND workspace_id = $2`, c.tx, c.adminId, c.newWsId); err != nil {
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableVersions+`
		WHERE admin_id = $1 AND workspace_id = $2`, c.tx, c.adminId, c.newWsId); err != nil {
		return err
	}
	usedNames := make(map[string]struct{}, len(rows))
	uniqueVersionName := func(baseName string) (string, error) {
		if _, exists := usedNames[baseName]; !exists {
			usedNames[baseName] = struct{}{}
			return baseName, nil
		}
		for suffixIndex := 2; suffixIndex <= len(rows)+1; suffixIndex++ {
			suffix := fmt.Sprintf(" (%d)", suffixIndex)
			nameRunes := []rune(baseName)
			maxNameRunes := 200 - len([]rune(suffix))
			if len(nameRunes) > maxNameRunes {
				nameRunes = nameRunes[:maxNameRunes]
			}
			candidate := string(nameRunes) + suffix
			if _, exists := usedNames[candidate]; exists {
				continue
			}
			usedNames[candidate] = struct{}{}
			return candidate, nil
		}
		return ``, fmt.Errorf("cannot allocate unique copied iteration name")
	}
	for _, row := range rows {
		versionName, err := uniqueVersionName(row[`name`])
		if err != nil {
			return err
		}
		ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableVersions+`
			(admin_id, workspace_id, name, start_date, end_date, ended_at, requirement_count, completed_count, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,0,0,$7,$8) RETURNING id`, c.tx,
			c.adminId, c.newWsId, versionName, row[`start_date`], row[`end_date`], row[`ended_at`], c.now, c.now)
		if err != nil {
			return err
		}
		newVersionId := cast.ToInt(ids[0][`id`])
		c.mappings.versions[cast.ToInt(row[`id`])] = newVersionId
		// Since the cross-workspace iteration model, iterations are resolved per
		// workspace through version_workspaces, so the association must be copied too.
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableVersionWorkspaces+`
			(admin_id, version_id, workspace_id, created_at)
			VALUES ($1,$2,$3,$4)`, c.tx, c.adminId, newVersionId, c.newWsId, c.now); err != nil {
			return err
		}
	}
	return nil
}

// ======================== requirement_categories ========================

func (c *TemplateConfigCopier) copyCategories() error {
	rows, err := c.sourceRows(SrcRequirementCategories)
	if err != nil {
		return err
	}
	// Insert every category as a root first, then remap parents in a second pass:
	// drag re-parenting can point a child at a later-created category, so the parent's
	// new id must not rely on row processing order.
	copiedIds := make(map[int]int, len(rows))
	parentRefs := make([]copiedParentRef, 0)
	for _, row := range rows {
		oldId := cast.ToInt(row[`id`])
		ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableRequirementCategories+`
			(admin_id, workspace_id, parent_id, name, sort_order, created_at, updated_at)
			VALUES ($1,$2,0,$3,$4,$5,$6) RETURNING id`, c.tx,
			c.adminId, c.newWsId, row[`name`], row[`sort_order`], c.now, c.now)
		if err != nil {
			return err
		}
		newId := cast.ToInt(ids[0][`id`])
		copiedIds[oldId] = newId
		if oldParent := cast.ToInt(row[`parent_id`]); oldParent > 0 {
			parentRefs = append(parentRefs, copiedParentRef{newId: newId, oldParent: oldParent})
		}
	}
	if err := c.remapCopiedParents(define.TableRequirementCategories, parentRefs, copiedIds); err != nil {
		return err
	}
	c.mappings.categories = copiedIds
	return nil
}

// ======================== automation_rules ========================

// copyAutomationRules copies the template workspace's automation rules. Copied rules are written
// disabled with a zeroed trigger count, the same convention as the in-product "copy this rule to
// another project" action, because a rule can still reference objects the new workspace does not
// have and the editor is where an administrator reviews them.
// A rule whose config no longer decodes is skipped: without a decoded config none of its
// references could be translated, so copying it verbatim would store template-workspace row ids
// as if they belonged to the new project.
func (c *TemplateConfigCopier) copyAutomationRules() error {
	// The evaluator enqueues in this order, so the copied rules keep their relative priority.
	rows, err := c.sourceRows(SrcAutomationRules)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableAutomationRules+`
		WHERE admin_id = $1 AND workspace_id = $2`, c.tx, c.adminId, c.newWsId); err != nil {
		logs.Error("delete new workspace automation rules err:%s", err.Error())
		return err
	}
	creator, err := automationUserSnapshotWithTx(c.tx, c.adminId, c.userId)
	if err != nil {
		return err
	}
	for _, row := range rows {
		config, err := decodeAutomationConfig([]byte(row[`config_json`]))
		if err != nil {
			logs.Error("copy template [%d] skips automation rule [%d]: %s", c.templateWsId, cast.ToInt(row[`id`]), err.Error())
			continue
		}
		remapCopiedAutomationConfig(&config, c.mappings, c.templateWsId, c.newWsId)
		encoded, err := json.Marshal(config)
		if err != nil {
			return err
		}
		locale := row[`locale`]
		if locale != lib_i18n.LangZhCn && locale != lib_i18n.LangEnUs {
			locale = lib_i18n.LangZhCn
		}
		if _, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableAutomationRules+`
			(admin_id, workspace_id, name, enabled, created_by, creator_name_snapshot, creator_avatar_snapshot,
			 locale, total_trigger_count, config_json, created_at, updated_at)
			VALUES ($1,$2,$3,0,$4,$5,$6,$7,0,$8,$9,$9)`, c.tx,
			c.adminId, c.newWsId, row[`name`], c.userId, creator.Name, creator.Avatar,
			locale, string(encoded), c.now); err != nil {
			logs.Error("copy automation rule err:%s", err.Error())
			return err
		}
	}
	return nil
}

// remapCopiedAutomationConfig rewrites every status, priority and iteration row id a copied rule
// compares or writes, so the references resolve against the copied rows instead of the template
// workspace. Field keys need no translation: built-in keys are fixed text and custom field keys
// are copied verbatim, along with their option keys. Member ids and project ids outside the
// copied set have no counterpart to map onto; the executor re-checks each of them against the
// running workspace and skips the action when the reference turns out to be stale.
func remapCopiedAutomationConfig(config *AutomationConfig, mappings templateIDMappings, templateWsId, newWsId int) {
	config.Trigger.FromStatusID = remapCopiedAutomationID(config.Trigger.FromStatusID, mappings.statuses)
	config.Trigger.ToStatusID = remapCopiedAutomationID(config.Trigger.ToStatusID, mappings.statuses)
	config.Trigger.FieldValue = remapCopiedAutomationValue(config.Trigger.FieldKey, config.Trigger.FieldValue, mappings)
	for _, node := range config.Nodes {
		for _, branch := range automationConditionBranches(node) {
			for _, module := range branch.Modules {
				for i := range module.Conditions {
					module.Conditions[i].Value = remapCopiedAutomationValue(
						module.Conditions[i].FieldKey, module.Conditions[i].Value, mappings)
				}
			}
			for i := range branch.Actions {
				remapCopiedAutomationAction(&branch.Actions[i], mappings, templateWsId, newWsId)
			}
		}
		if node.Action != nil {
			remapCopiedAutomationAction(node.Action, mappings, templateWsId, newWsId)
		}
	}
}

// remapCopiedAutomationAction rewrites the references one action compares or creates.
func remapCopiedAutomationAction(action *AutomationAction, mappings templateIDMappings, templateWsId, newWsId int) {
	for i := range action.Transitions {
		action.Transitions[i].TargetStatusID = remapCopiedAutomationID(action.Transitions[i].TargetStatusID, mappings.statuses)
	}
	for i := range action.Updates {
		action.Updates[i].Value = remapCopiedAutomationValue(action.Updates[i].FieldKey, action.Updates[i].Value, mappings)
	}
	for _, field := range []struct {
		value   *AutomationCreateField
		mapping map[int]int
	}{
		{action.Status, mappings.statuses},
		{action.Priority, mappings.priorities},
		{action.Version, mappings.versions},
	} {
		if field.value == nil || field.value.Source != `fixed` {
			continue
		}
		field.value.Value = remapCopiedAutomationID(field.value.Value, field.mapping)
	}
	// A rule that copied its work item back into the template project targets the new one instead;
	// any other project stays as configured, since the copy owns no mapping for it.
	if action.Type == `copy_work_item` && action.TargetWorkspaceID == templateWsId {
		action.TargetWorkspaceID = newWsId
	}
}

// remapCopiedAutomationValue rewrites the row ids a status, priority or iteration condition or
// fixed update compares against, including the id list an `in` or `not_in` condition keeps. Every
// list element is translated on its own and keeps its stored JSON type. Every other value kind
// (text, date keyword, custom option key) is portable as it is.
func remapCopiedAutomationValue(fieldKey string, raw json.RawMessage, mappings templateIDMappings) json.RawMessage {
	var mapping map[int]int
	switch fieldKey {
	case `status`:
		mapping = mappings.statuses
	case `priority`:
		mapping = mappings.priorities
	case `version`:
		mapping = mappings.versions
	default:
		return raw
	}
	value := rawJSONValue(raw)
	if mapped := remapNumericID(value, mapping); mapped != `` {
		return json.RawMessage(mapped)
	}
	values, ok := value.([]any)
	if !ok {
		return raw
	}
	mappedValues := make([]any, len(values))
	changed := false
	for i, item := range values {
		mappedValues[i] = item
		newId := remapNumericID(item, mapping)
		if newId == `` {
			continue
		}
		changed = true
		if _, isText := item.(string); isText {
			mappedValues[i] = newId
		} else {
			mappedValues[i] = json.Number(newId)
		}
	}
	if !changed {
		return raw
	}
	encoded, err := json.Marshal(mappedValues)
	if err != nil {
		return raw
	}
	return encoded
}

// remapCopiedAutomationID returns the copied row id for an id of the template workspace, or the
// original id when no copy exists for it.
func remapCopiedAutomationID(id int, mapping map[int]int) int {
	if mapped, ok := mapping[id]; ok && mapped > 0 {
		return mapped
	}
	return id
}

// copyDashboards copies the template project's shared dashboards, role visibility, cards and the
// project announcement. It mirrors CopyDashboardProjectTemplateTx: dashboards are attributed to
// the operator, member visibility normalises to public (those users are not copied), chart and
// metric configs lose their project-scoped ids/filters, and text/announcement bodies go through
// the package asset migration. The destination is wiped first because enabling the dashboard
// application already seeded the starter board (and a default announcement) into the new
// workspace before this step runs.
func (c *TemplateConfigCopier) copyDashboards() error {
	dashboards, err := c.sourceRows(SrcDashboards)
	if err != nil {
		return err
	}
	cardsByDash, err := c.dashboardCardsBySource()
	if err != nil {
		return err
	}
	rolesByDash, err := c.dashboardRolesBySource()
	if err != nil {
		return err
	}
	announcementRows, err := c.sourceRows(SrcDashboardAnnouncements)
	if err != nil {
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableDashboardCards+`
		WHERE admin_id = $1 AND workspace_id = $2`, c.tx, c.adminId, c.newWsId); err != nil {
		logs.Error("delete new workspace dashboard cards err:%s", err.Error())
		return err
	}
	for _, table := range []string{define.TableWorkspaceDashboards, define.TableDashboardVisibilityRoles, define.TableDashboardAnnouncements} {
		if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+table+`
			WHERE admin_id = $1 AND workspace_id = $2`, c.tx, c.adminId, c.newWsId); err != nil {
			logs.Error("delete new workspace dashboard rows failed, table:%s, err:%s", table, err.Error())
			return err
		}
	}
	assetCache := make(map[string]copiedAssetInfo)
	firstName := ``
	for _, row := range dashboards {
		if firstName == `` {
			firstName = row[`name`]
		}
		newId, err := c.insertCopiedDashboard(row)
		if err != nil {
			return err
		}
		if err := c.insertCopiedDashboardRoles(row, newId, rolesByDash[cast.ToInt(row[`id`])]); err != nil {
			return err
		}
		if err := c.copyCopiedDashboardCards(cardsByDash[cast.ToInt(row[`id`])], newId, assetCache); err != nil {
			return err
		}
	}
	return c.copyCopiedDashboardAnnouncement(announcementRows, firstName, assetCache)
}

// insertCopiedDashboard writes one packaged board owned by the operator. An unparsed visibility
// value is skipped by the caller, never written.
func (c *TemplateConfigCopier) insertCopiedDashboard(row msql.Params) (int, error) {
	visibility := row[`visibility_type`]
	if visibility == define.DashboardVisibilityMembers {
		// Member visibility references template-account users that never arrive in the new
		// project, so the board opens to everyone, same as the live project-template copy.
		visibility = define.DashboardVisibilityPublic
	}
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkspaceDashboards+`
		(admin_id, workspace_id, name, visibility_type, sort_order, creator_id, revision, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,1,$7,$7) RETURNING id`, c.tx,
		c.adminId, c.newWsId, row[`name`], visibility, row[`sort_order`], c.userId, c.now)
	if err != nil {
		logs.Error("copy template [%d] insert dashboard err:%s", c.templateWsId, err.Error())
		return 0, err
	}
	return cast.ToInt(rows[0][`id`]), nil
}

// insertCopiedDashboardRoles remaps a role-visible board's roles through the roles step's map.
// Roles the copy did not carry are dropped; a board left without any visible role would be
// invisible to every member, so it degrades to public instead.
func (c *TemplateConfigCopier) insertCopiedDashboardRoles(row msql.Params, newDashboardId int, oldRoleIds []int) error {
	if row[`visibility_type`] != define.DashboardVisibilityRoles {
		return nil
	}
	seen := make(map[int]bool, len(oldRoleIds))
	roleIds := make([]int, 0, len(oldRoleIds))
	for _, oldId := range oldRoleIds {
		newId := c.mappings.roles[oldId]
		if newId <= 0 || seen[newId] {
			continue
		}
		seen[newId] = true
		roleIds = append(roleIds, newId)
	}
	if len(roleIds) == 0 {
		logs.Warning("copy template [%d] dashboard [%s] has no copiable visible role, opened as public", c.templateWsId, row[`id`])
		if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceDashboards+`
			SET visibility_type = $3 WHERE admin_id = $1 AND workspace_id = $2 AND id = $4`,
			c.tx, c.adminId, c.newWsId, define.DashboardVisibilityPublic, newDashboardId); err != nil {
			logs.Error("open copied dashboard as public err:%s", err.Error())
			return err
		}
		return nil
	}
	for _, roleId := range roleIds {
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableDashboardVisibilityRoles+`
			(admin_id, workspace_id, dashboard_id, role_id, created_at)
			VALUES ($1,$2,$3,$4,$5)`, c.tx, c.adminId, c.newWsId, newDashboardId, roleId, c.now); err != nil {
			logs.Error("copy dashboard visibility role err:%s", err.Error())
			return err
		}
	}
	return nil
}

// dashboardCardsBySource groups the packaged cards under their source dashboard. The package
// order (dashboard_id, grid_y, grid_x, id) already matches the live board rendering order.
func (c *TemplateConfigCopier) dashboardCardsBySource() (map[int][]msql.Params, error) {
	rows, err := c.sourceRows(SrcDashboardCards)
	if err != nil {
		return nil, err
	}
	grouped := make(map[int][]msql.Params)
	for _, row := range rows {
		key := cast.ToInt(row[`dashboard_id`])
		grouped[key] = append(grouped[key], row)
	}
	return grouped, nil
}

// dashboardRolesBySource groups the packaged role visibility rows under their source dashboard.
func (c *TemplateConfigCopier) dashboardRolesBySource() (map[int][]int, error) {
	rows, err := c.sourceRows(SrcDashboardVisibilityRoles)
	if err != nil {
		return nil, err
	}
	grouped := make(map[int][]int)
	for _, row := range rows {
		grouped[cast.ToInt(row[`dashboard_id`])] = append(grouped[cast.ToInt(row[`dashboard_id`])], cast.ToInt(row[`role_id`]))
	}
	return grouped, nil
}

// copyCopiedDashboardCards writes one board's packaged cards. Cards over the live per-board
// limit, with an unparsable config or an unknown type are skipped: the board still arrives, only
// the broken card is lost.
func (c *TemplateConfigCopier) copyCopiedDashboardCards(rows []msql.Params, newDashboardId int, assetCache map[string]copiedAssetInfo) error {
	count := 0
	for _, row := range rows {
		card := DashboardCardModel{
			CardType:    row[`card_type`],
			Title:       row[`title`],
			Config:      row[`config`],
			Content:     row[`content`],
			ContentMode: define.DashboardContentModeRichText,
			X:           cast.ToInt(row[`grid_x`]),
			Y:           cast.ToInt(row[`grid_y`]),
			W:           cast.ToInt(row[`grid_width`]),
			H:           cast.ToInt(row[`grid_height`]),
		}
		switch card.CardType {
		case define.DashboardCardTypeChart:
			normalized, err := normalizeCopiedChartConfig(card.Config)
			if err != nil {
				logs.Warning("copy template [%d] skips dashboard card [%s]: %s", c.templateWsId, row[`id`], err.Error())
				continue
			}
			card.Config = normalized
		case define.DashboardCardTypeMetric:
			normalized, err := normalizeCopiedMetricConfig(card.Config)
			if err != nil {
				logs.Warning("copy template [%d] skips dashboard card [%s]: %s", c.templateWsId, row[`id`], err.Error())
				continue
			}
			card.Config = normalized
		case define.DashboardCardTypeProjectInfo, define.DashboardCardTypeAnnouncement, define.DashboardCardTypeText:
		default:
			logs.Warning("copy template [%d] skips dashboard card [%s] of unknown type [%s]", c.templateWsId, row[`id`], card.CardType)
			continue
		}
		if count >= define.DashboardMaxCardsPerDashboard {
			logs.Warning("copy template [%d] dashboard [%d] exceeds the %d card limit, later cards skipped",
				c.templateWsId, newDashboardId, define.DashboardMaxCardsPerDashboard)
			break
		}
		created, err := insertDashboardCardTx(c.tx, c.adminId, c.newWsId, newDashboardId, c.userId, card, int64(c.now))
		if err != nil {
			return err
		}
		count++
		if card.CardType != define.DashboardCardTypeText {
			continue
		}
		content, assets, err := c.migrateContentAssets(row[`content`], assetCache)
		if err != nil {
			return err
		}
		if err := c.bookCopiedDashboardAssets(assets, define.SourceTypeDashboardCard, created.Id, card.Title); err != nil {
			return err
		}
		if content == row[`content`] {
			continue
		}
		if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableDashboardCards+`
			SET content = $1, updated_at = $2 WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`,
			c.tx, content, c.now, c.adminId, c.newWsId, created.Id); err != nil {
			logs.Error("write copied dashboard card content err:%s", err.Error())
			return err
		}
	}
	return nil
}

// normalizeCopiedChartConfig clears the chart filters that reference the template project's own
// status/priority rows, exactly like the live project-template copy; a config that no longer
// decodes is an error so the caller skips the card.
func normalizeCopiedChartConfig(raw string) (string, error) {
	var config DashboardChartConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return ``, fmt.Errorf("chart config is not valid JSON: %w", err)
	}
	config.StatusIds = nil
	config.PriorityIds = nil
	config.CustomOrder = nil
	config.SortOrder = "default"
	encoded, err := json.Marshal(config)
	if err != nil {
		return ``, err
	}
	return string(encoded), nil
}

// normalizeCopiedMetricConfig drops the metric card's filters, whose condition ids belong to the
// template project and have no counterpart here.
func normalizeCopiedMetricConfig(raw string) (string, error) {
	var config DashboardMetricConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return ``, fmt.Errorf("metric config is not valid JSON: %w", err)
	}
	config.Filters = json.RawMessage(`null`)
	encoded, err := json.Marshal(config)
	if err != nil {
		return ``, err
	}
	return string(encoded), nil
}

// copyCopiedDashboardAnnouncement writes the project announcement. The package carries at most
// one row; a workspace without one keeps the seeded default the dashboard application enabled.
func (c *TemplateConfigCopier) copyCopiedDashboardAnnouncement(rows []msql.Params, boardName string, assetCache map[string]copiedAssetInfo) error {
	if len(rows) == 0 {
		return nil
	}
	row := rows[0]
	ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableDashboardAnnouncements+`
		(admin_id, workspace_id, content, content_mode, revision, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,1,$5,$5,$6,$6) RETURNING id`, c.tx,
		c.adminId, c.newWsId, row[`content`], define.DashboardContentModeRichText, c.userId, c.now)
	if err != nil {
		logs.Error("copy template [%d] insert dashboard announcement err:%s", c.templateWsId, err.Error())
		return err
	}
	newId := cast.ToInt(ids[0][`id`])
	content, assets, err := c.migrateContentAssets(row[`content`], assetCache)
	if err != nil {
		return err
	}
	if err := c.bookCopiedDashboardAssets(assets, define.SourceTypeDashboardAnnouncement, newId, boardName); err != nil {
		return err
	}
	if content == row[`content`] {
		return nil
	}
	if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableDashboardAnnouncements+`
		SET content = $1, updated_at = $2 WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`,
		c.tx, content, c.now, c.adminId, c.newWsId, newId); err != nil {
		logs.Error("write copied dashboard announcement content err:%s", err.Error())
		return err
	}
	return nil
}

// bookCopiedDashboardAssets records migrated rich-text images on the destination file ledger
// bound to the copied card/announcement row. migrateContentAssets deduplicates through the shared
// cache across the whole dashboard copy, so every object is uploaded and booked exactly once.
func (c *TemplateConfigCopier) bookCopiedDashboardAssets(assets []copiedAssetInfo, sourceType string, sourceId int, sourceName string) error {
	for _, asset := range assets {
		if _, err := c.insertCopiedAssetLedger(copiedAssetBinding{
			sourceType: sourceType,
			sourceID:   sourceId,
			sourceName: sourceName,
		}, asset); err != nil {
			return err
		}
	}
	return nil
}

// ======================== roles ========================

func (c *TemplateConfigCopier) copyRoles() error {
	rows, err := c.sourceRows(SrcRoles)
	if err != nil {
		return err
	}
	newMemberRoleId := c.findNewMemberRoleId()
	if newMemberRoleId == 0 {
		return fmt.Errorf("new workspace member role not found")
	}
	// System role names are stored wrapped (e.g. ${perm_role_project_member}); match
	// the wrapped form like the rest of the codebase (workspace.go seeds, owner_transfer.go).
	adminRoleName := lib_i18n.WrapKey(`perm_role_project_admin`)
	memberRoleName := lib_i18n.WrapKey(`perm_role_project_member`)
	for _, row := range rows {
		isSystem := cast.ToInt(row[`is_system`])
		roleName := row[`name`]
		if isSystem == 1 && roleName == adminRoleName {
			continue
		}
		if isSystem == 1 && roleName == memberRoleName {
			if err := c.syncRolePermissions(row, newMemberRoleId); err != nil {
				return err
			}
			if err := c.syncMemberRoleDataScope(row, newMemberRoleId); err != nil {
				return err
			}
			c.mappings.roles[cast.ToInt(row[`id`])] = newMemberRoleId
			continue
		}
		newRoleId, err := c.insertCustomRole(row)
		if err != nil {
			return err
		}
		c.mappings.roles[cast.ToInt(row[`id`])] = newRoleId
		if err := c.syncRolePermissions(row, newRoleId); err != nil {
			return err
		}
	}
	return nil
}

func (c *TemplateConfigCopier) findNewMemberRoleId() int {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableRoles+`
		WHERE admin_id = $1 AND workspace_id = $2 AND name = $3 AND is_system = 1`,
		c.tx, c.adminId, c.newWsId, lib_i18n.WrapKey(`perm_role_project_member`))
	if err != nil {
		logs.Error("find new member role id err:%s", err.Error())
		return 0
	}
	if len(rows) == 0 {
		return 0
	}
	return cast.ToInt(rows[0][`id`])
}

func (c *TemplateConfigCopier) insertCustomRole(templateRole msql.Params) (int, error) {
	ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableRoles+`
		(admin_id, workspace_id, name, description, is_system, scope, data_view_scope, created_at, updated_at)
		VALUES ($1,$2,$3,$4,0,'project',$5,$6,$7) RETURNING id`, c.tx,
		c.adminId, c.newWsId, templateRole[`name`], templateRole[`description`],
		copiedDataViewScope(templateRole[`data_view_scope`]), c.now, c.now)
	if err != nil {
		return 0, err
	}
	return cast.ToInt(ids[0][`id`]), nil
}

// copiedDataViewScope preserves the data view scope value. Partial scopes are kept as-is;
// the role_data_view_members rows for the template workspace's users are not copied since
// those users belong to a different tenant, so the role will have partial scope with an
// empty member set — the workspace admin can configure members after creation.
func copiedDataViewScope(scope string) string {
	return scope
}

// syncMemberRoleDataScope copies the template member role's data view scope onto the
// new workspace's seeded member role, matching the scope handling of custom roles.
func (c *TemplateConfigCopier) syncMemberRoleDataScope(templateRole msql.Params, newMemberRoleId int) error {
	if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableRoles+` SET
		data_view_scope = $3, updated_at = $4
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $5`, c.tx,
		c.adminId, c.newWsId, copiedDataViewScope(templateRole[`data_view_scope`]), c.now, newMemberRoleId); err != nil {
		logs.Error("update member role data view scope err:%s", err.Error())
		return err
	}
	return nil
}

// syncRolePermissions replaces the destination role's permissions with the package role's.
// A role without the derived key carries no permissions; a malformed value is an error, since
// silently copying an empty permission set would quietly strip the role.
func (c *TemplateConfigCopier) syncRolePermissions(templateRole msql.Params, targetRoleId int) error {
	keys, err := templateRolePermissionKeys(templateRole)
	if err != nil {
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableRolePermissions+`
		WHERE admin_id = $1 AND role_id = $2`, c.tx, c.adminId, targetRoleId); err != nil {
		logs.Error("delete role permissions err:%s", err.Error())
		return err
	}
	for _, key := range keys {
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableRolePermissions+`
			(admin_id, role_id, permission_key, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5)`, c.tx,
			c.adminId, targetRoleId, key, c.now, c.now); err != nil {
			return err
		}
	}
	return nil
}

// templateRolePermissionKeys decodes the permission key array the exporter folded into a role
// row. The order is the package's, which is the export query's `ORDER BY id ASC`. Keys this
// build does not register are dropped: the package carries the template account's rows verbatim
// and may come from another version, and an unregistered key grants nothing while every
// authorization check would reject it.
func templateRolePermissionKeys(templateRole msql.Params) ([]string, error) {
	raw := templateRole[TemplateDerivedRolePermissions]
	if raw == `` {
		return nil, nil
	}
	var keys []string
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		logs.Error("decode template role [%s] permissions err:%s", templateRole[`id`], err.Error())
		return nil, err
	}
	registered := make([]string, 0, len(keys))
	for _, key := range keys {
		if _, ok := permission.GetPermission(key); !ok {
			logs.Warning("copy template role [%s] permission [%s] is not registered, skipped", templateRole[`id`], key)
			continue
		}
		registered = append(registered, key)
	}
	return registered, nil
}
