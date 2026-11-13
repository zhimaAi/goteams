// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package permission

import (
	"strings"

	"goteams/internal/pkg/lib_i18n"
)

// ======================== Permission System Design ========================
//
// Permission definitions are hardcoded in code (single source of truth),
// while role → permission mappings are stored in the database.
//
// Three-layer structure:
//   Permission Group → Sub-permission → API
//
// Frontend: uses Group Key + Permission Key to control UI display (button/menu visibility).
// Backend: uses Permission Key to validate API access (middleware interception).
// i18n: Name is an i18n key; use T(lang) to get the display text for the target language.

// Permission represents a single permission definition.
type Permission struct {
	Key   string // Unique identifier, e.g. "requirement:create".
	Name  string // i18n key, e.g. "perm_requirement_create".
	Group string // Parent group's Group.Key.
}

// Group represents a permission group (for frontend grouped display).
type Group struct {
	Key         string       // Unique identifier, e.g. "workspace".
	Name        string       // i18n key, e.g. "perm_group_workspace".
	Permissions []Permission // Permission list within the group.
}

// ======================== Permission Group Key Constants ========================

const (
	GroupKeyProject           = "project"
	GroupKeyTeamWork          = "team_work"
	GroupKeyWorkspace         = "workspace"
	GroupKeyRequirement       = "requirement"
	GroupKeyDefect            = "defect"
	GroupKeyGantt             = "gantt"
	GroupKeyVersion           = "version"
	GroupKeyPlan              = "plan"
	GroupKeyMemberEnterprise  = "members_enterprise"
	GroupKeyMemberRoles       = "members_roles"
	GroupKeyProjectInfo       = "project_settings_info"
	GroupKeyProjectMembers    = "project_settings_members"
	GroupKeyProjectRoles      = "project_settings_roles"
	GroupKeyRequirementSetup  = "project_settings_requirement"
	GroupKeyDefectSetup       = "project_settings_defect"
	GroupKeyStorage           = "storage"
	GroupKeySso               = "sso"
	GroupKeyPersonalization   = "personalization"
	GroupKeyAutomation        = "project_settings_automation"
	GroupKeySLA               = "workspace_settings_sla"
	GroupKeyApplicationMarket = "workspace_settings_applications"
	GroupKeyPlanning          = "planning"
	GroupKeyPipeline          = "pipeline"
	GroupKeyKnowledgeBase     = "kb"
	GroupKeyExternalNotify    = "external_notification"
	GroupKeyDashboard         = "dashboard"
)

// ======================== All Permission Hardcoded Definitions ========================

// AllPermissions is a flat list of all permissions (for key → Permission lookup).
// The Name field is an i18n key, corresponding to perm_xxx entries in locale_*.ini.
var AllPermissions = []Permission{
	{Key: "sla:view", Name: "perm_sla_view", Group: GroupKeySLA},
	{Key: "sla:manage", Name: "perm_sla_manage", Group: GroupKeySLA},
	// ---- Team Work ----
	{Key: "team_work:view_today", Name: "perm_team_work_view_today", Group: GroupKeyTeamWork},
	{Key: "team_work:view_yesterday", Name: "perm_team_work_view_yesterday", Group: GroupKeyTeamWork},
	{Key: "team_work:view_tomorrow", Name: "perm_team_work_view_tomorrow", Group: GroupKeyTeamWork},
	{Key: "team_work:view_custom_date", Name: "perm_team_work_view_custom_date", Group: GroupKeyTeamWork},
	{Key: "team_work:view_unscheduled", Name: "perm_team_work_view_unscheduled", Group: GroupKeyTeamWork},
	{Key: "team_work:view_scheduled", Name: "perm_team_work_view_scheduled", Group: GroupKeyTeamWork},

	// ---- Project ----
	{Key: "project:create", Name: "perm_project_create", Group: GroupKeyProject},
	{Key: "project:manage", Name: "perm_project_manage", Group: GroupKeyProject},

	// ---- Enterprise Plan ----
	{Key: "plan:view", Name: "perm_plan_view", Group: GroupKeyPlan},

	// ---- Workspace ----
	{Key: "workspace:view", Name: "perm_workspace_view", Group: GroupKeyWorkspace},
	{Key: "workspace:edit", Name: "perm_workspace_edit", Group: GroupKeyWorkspace},
	// workspace:end ends/reopens a project; permanent deletion is covered by the enterprise-level project:manage.
	{Key: "workspace:end", Name: "perm_workspace_end", Group: GroupKeyWorkspace},

	// ---- Requirement ----
	{Key: "requirement:view", Name: "perm_requirement_view", Group: GroupKeyRequirement},
	{Key: "requirement:create", Name: "perm_requirement_create", Group: GroupKeyRequirement},
	{Key: "requirement:edit", Name: "perm_requirement_edit", Group: GroupKeyRequirement},
	{Key: "requirement:delete", Name: "perm_requirement_delete", Group: GroupKeyRequirement},
	{Key: "requirement:import", Name: "perm_requirement_import", Group: GroupKeyRequirement},
	{Key: "requirement:export", Name: "perm_requirement_export", Group: GroupKeyRequirement},
	{Key: "requirement:copy", Name: "perm_requirement_copy", Group: GroupKeyRequirement},
	{Key: "requirement:manage_system_view", Name: "perm_requirement_manage_system_view", Group: GroupKeyRequirement},
	{Key: "requirement:manage_category", Name: "perm_requirement_manage_category", Group: GroupKeyRequirement},

	// ---- Defect ----
	{Key: "defect:view", Name: "perm_defect_view", Group: GroupKeyDefect},
	{Key: "defect:create", Name: "perm_defect_create", Group: GroupKeyDefect},
	{Key: "defect:edit", Name: "perm_defect_edit", Group: GroupKeyDefect},
	{Key: "defect:delete", Name: "perm_defect_delete", Group: GroupKeyDefect},
	{Key: "defect:import", Name: "perm_defect_import", Group: GroupKeyDefect},
	{Key: "defect:export", Name: "perm_defect_export", Group: GroupKeyDefect},
	{Key: "defect:copy", Name: "perm_defect_copy", Group: GroupKeyDefect},
	{Key: "defect:manage_system_view", Name: "perm_defect_manage_system_view", Group: GroupKeyDefect},

	// Gantt chart permissions (workspace scope).
	{Key: "gantt:view", Name: "perm_gantt_view", Group: GroupKeyGantt},
	{Key: "gantt:manage_system_view", Name: "perm_gantt_manage_system_view", Group: GroupKeyGantt},

	// ---- Version ----
	{Key: "version:view", Name: "perm_version_view", Group: GroupKeyVersion},
	{Key: "version:create", Name: "perm_version_create", Group: GroupKeyVersion},
	{Key: "version:edit", Name: "perm_version_edit", Group: GroupKeyVersion},
	{Key: "version:delete", Name: "perm_version_delete", Group: GroupKeyVersion},
	{Key: "version:manage_system_view", Name: "perm_version_manage_system_view", Group: GroupKeyVersion},

	// ---- Members > Enterprise Members ----
	{Key: "member:view", Name: "perm_member_view", Group: GroupKeyMemberEnterprise},
	{Key: "member:invite", Name: "perm_member_invite", Group: GroupKeyMemberEnterprise},
	{Key: "member:remove", Name: "perm_member_remove", Group: GroupKeyMemberEnterprise},
	{Key: "member:edit", Name: "perm_member_edit", Group: GroupKeyMemberEnterprise},
	{Key: "member:reset_password", Name: "perm_member_reset_password", Group: GroupKeyMemberEnterprise},

	// ---- Members > Enterprise Roles ----
	{Key: "member:view_role_list", Name: "perm_member_view_role_list", Group: GroupKeyMemberRoles},
	{Key: "member:create_role", Name: "perm_member_create_role", Group: GroupKeyMemberRoles},
	{Key: "member:delete_role", Name: "perm_member_delete_role", Group: GroupKeyMemberRoles},
	{Key: "member:edit_role_permissions", Name: "perm_member_edit_role_permissions", Group: GroupKeyMemberRoles},

	// ---- Project Settings ----
	{Key: "project_info:edit", Name: "perm_project_info_edit", Group: GroupKeyProjectInfo},
	{Key: "project_member:add", Name: "perm_project_member_add", Group: GroupKeyProjectMembers},
	{Key: "project_member:remove", Name: "perm_project_member_remove", Group: GroupKeyProjectMembers},
	{Key: "project_member:change_role", Name: "perm_project_member_change_role", Group: GroupKeyProjectMembers},
	{Key: "project_role:add", Name: "perm_project_role_add", Group: GroupKeyProjectRoles},
	{Key: "project_role:delete", Name: "perm_project_role_delete", Group: GroupKeyProjectRoles},
	{Key: "project_role:edit", Name: "perm_project_role_edit", Group: GroupKeyProjectRoles},
	{Key: "requirement_setting:field", Name: "perm_requirement_setting_field", Group: GroupKeyRequirementSetup},
	{Key: "requirement_setting:status", Name: "perm_requirement_setting_status", Group: GroupKeyRequirementSetup},
	{Key: "requirement_setting:priority", Name: "perm_requirement_setting_priority", Group: GroupKeyRequirementSetup},
	{Key: "requirement_setting:attribute", Name: "perm_requirement_setting_attribute", Group: GroupKeyRequirementSetup},
	{Key: "defect_setting:field", Name: "perm_defect_setting_field", Group: GroupKeyDefectSetup},
	{Key: "defect_setting:status", Name: "perm_defect_setting_status", Group: GroupKeyDefectSetup},
	{Key: "defect_setting:priority", Name: "perm_defect_setting_priority", Group: GroupKeyDefectSetup},
	{Key: "defect_setting:attribute", Name: "perm_defect_setting_attribute", Group: GroupKeyDefectSetup},

	// ---- Storage ----
	{Key: "storage:view", Name: "perm_storage_view", Group: GroupKeyStorage},
	{Key: "storage:download", Name: "perm_storage_download", Group: GroupKeyStorage},
	{Key: "storage:delete", Name: "perm_storage_delete", Group: GroupKeyStorage},

	{Key: "application_market:view", Name: "perm_application_market_view", Group: GroupKeyApplicationMarket},
	{Key: "application_market:manage", Name: "perm_application_market_manage", Group: GroupKeyApplicationMarket},

	// ---- Automation ----
	{Key: "automation:view", Name: "perm_automation_view", Group: GroupKeyAutomation},
	{Key: "automation:manage", Name: "perm_automation_manage", Group: GroupKeyAutomation},

	// ---- Enterprise SSO ----
	{Key: "sso:setting", Name: "perm_sso_setting", Group: GroupKeySso},
	{Key: "personalization:setting", Name: "perm_personalization_setting", Group: GroupKeyPersonalization},

	// ---- Planning ----
	// The planning module only has view and edit permissions; all write operations (groups, row data, column order,
	// and planning config in project settings) are uniformly controlled by planning:edit.
	{Key: "planning:view", Name: "perm_planning_view", Group: GroupKeyPlanning},
	{Key: "planning:edit", Name: "perm_planning_edit", Group: GroupKeyPlanning},

	// ---- Pipeline ----
	{Key: "pipeline:view", Name: "perm_pipeline_view", Group: GroupKeyPipeline},
	{Key: "pipeline:manage", Name: "perm_pipeline_manage", Group: GroupKeyPipeline},

	// ---- Knowledge Base ----
	{Key: "kb:view", Name: "perm_kb_view", Group: GroupKeyKnowledgeBase},
	{Key: "kb:create", Name: "perm_kb_create", Group: GroupKeyKnowledgeBase},
	{Key: "kb:manage", Name: "perm_kb_manage", Group: GroupKeyKnowledgeBase},
	{Key: "external_notification:manage", Name: "perm_external_notification_manage", Group: GroupKeyExternalNotify},

	// ---- Project Dashboard ----
	{Key: "dashboard:view", Name: "perm_dashboard_view", Group: GroupKeyDashboard},
	{Key: "dashboard:create", Name: "perm_dashboard_create", Group: GroupKeyDashboard},

	// ---- Issue Invitation ----
	{Key: "ticket:copy_link", Name: "perm_ticket_copy_link", Group: GroupKeyTicket},
	{Key: "ticket:manage", Name: "perm_ticket_manage", Group: GroupKeyTicket},
}

// PermissionMap provides fast lookup: key → Permission.
var PermissionMap = buildPermissionMap()

func buildPermissionMap() map[string]Permission {
	m := make(map[string]Permission, len(AllPermissions))
	for _, p := range AllPermissions {
		m[p.Key] = p
	}
	return m
}

// GetPermission returns the permission definition by key.
func GetPermission(key string) (Permission, bool) {
	p, ok := PermissionMap[key]
	return p, ok
}

// GetPermissionName returns the i18n display name for a permission (based on language).
func GetPermissionName(key, lang string) string {
	p, ok := PermissionMap[key]
	if !ok {
		return key
	}
	return lib_i18n.Show(lang, p.Name)
}

// ======================== Permission Groups (for frontend display) ========================

// AllGroups contains all permission groups (in display order); Name is an i18n key.
var AllGroups = []Group{
	{
		Key:  GroupKeyTeamWork,
		Name: "perm_group_team_work",
		Permissions: []Permission{
			PermissionMap["team_work:view_today"],
			PermissionMap["team_work:view_yesterday"],
			PermissionMap["team_work:view_tomorrow"],
			PermissionMap["team_work:view_custom_date"],
			PermissionMap["team_work:view_unscheduled"],
			PermissionMap["team_work:view_scheduled"],
		},
	},
	{
		Key:  GroupKeyProject,
		Name: "perm_group_project",
		Permissions: []Permission{
			PermissionMap["project:create"],
			PermissionMap["project:manage"],
		},
	},
	{
		Key:  GroupKeyPlan,
		Name: "perm_group_plan",
		Permissions: []Permission{
			PermissionMap["plan:view"],
		},
	},
	{
		Key:  GroupKeyMemberEnterprise,
		Name: "perm_group_members_enterprise",
		Permissions: []Permission{
			PermissionMap["member:view"],
			PermissionMap["member:invite"],
			PermissionMap["member:remove"],
			PermissionMap["member:edit"],
			PermissionMap["member:reset_password"],
		},
	},
	{
		Key:  GroupKeyMemberRoles,
		Name: "perm_group_members_roles",
		Permissions: []Permission{
			PermissionMap["member:view_role_list"],
			PermissionMap["member:create_role"],
			PermissionMap["member:delete_role"],
			PermissionMap["member:edit_role_permissions"],
		},
	},
	{
		Key:  GroupKeyExternalNotify,
		Name: "perm_group_external_notification",
		Permissions: []Permission{
			PermissionMap["external_notification:manage"],
		},
	},
	{
		Key:  GroupKeyWorkspace,
		Name: "perm_group_workspace",
		Permissions: []Permission{
			PermissionMap["workspace:view"],
			PermissionMap["workspace:edit"],
			PermissionMap["workspace:end"],
		},
	},
	{
		Key:  GroupKeyRequirement,
		Name: "perm_group_requirement",
		Permissions: []Permission{
			PermissionMap["requirement:view"],
			PermissionMap["requirement:create"],
			PermissionMap["requirement:delete"],
			PermissionMap["requirement:edit"],
			PermissionMap["requirement:import"],
			PermissionMap["requirement:export"],
			PermissionMap["requirement:copy"],
			PermissionMap["requirement:manage_system_view"],
			PermissionMap["requirement:manage_category"],
		},
	},
	{
		Key:  GroupKeyDefect,
		Name: "perm_group_defect",
		Permissions: []Permission{
			PermissionMap["defect:view"],
			PermissionMap["defect:create"],
			PermissionMap["defect:delete"],
			PermissionMap["defect:edit"],
			PermissionMap["defect:import"],
			PermissionMap["defect:export"],
			PermissionMap["defect:copy"],
			PermissionMap["defect:manage_system_view"],
		},
	},
	{
		Key:  GroupKeyGantt,
		Name: "perm_group_gantt",
		Permissions: []Permission{
			PermissionMap["gantt:view"],
			PermissionMap["gantt:manage_system_view"],
		},
	},
	{
		Key:  GroupKeyVersion,
		Name: "perm_group_version",
		Permissions: []Permission{
			PermissionMap["version:view"],
			PermissionMap["version:create"],
			PermissionMap["version:delete"],
			PermissionMap["version:edit"],
			PermissionMap["version:manage_system_view"],
		},
	},
	{
		Key:  GroupKeyKnowledgeBase,
		Name: "perm_group_kb",
		Permissions: []Permission{
			PermissionMap["kb:view"],
			PermissionMap["kb:create"],
			PermissionMap["kb:manage"],
		},
	},
	{
		Key:  GroupKeyDashboard,
		Name: "perm_group_dashboard",
		Permissions: []Permission{
			PermissionMap["dashboard:view"],
			PermissionMap["dashboard:create"],
		},
	},
	{
		Key:  GroupKeyTicket,
		Name: "perm_group_ticket",
		Permissions: []Permission{
			PermissionMap["ticket:copy_link"],
			PermissionMap["ticket:manage"],
		},
	},
	{
		Key:  GroupKeyProjectInfo,
		Name: "perm_group_project_info",
		Permissions: []Permission{
			PermissionMap["project_info:edit"],
		},
	},
	{
		Key:  GroupKeyProjectMembers,
		Name: "perm_group_project_members",
		Permissions: []Permission{
			PermissionMap["project_member:add"],
			PermissionMap["project_member:remove"],
			PermissionMap["project_member:change_role"],
		},
	},
	{
		Key:  GroupKeyProjectRoles,
		Name: "perm_group_project_roles",
		Permissions: []Permission{
			PermissionMap["project_role:add"],
			PermissionMap["project_role:delete"],
			PermissionMap["project_role:edit"],
		},
	},
	{
		Key:  GroupKeyRequirementSetup,
		Name: "perm_group_requirement_setup",
		Permissions: []Permission{
			PermissionMap["requirement_setting:attribute"],
			PermissionMap["requirement_setting:field"],
			PermissionMap["requirement_setting:status"],
			PermissionMap["requirement_setting:priority"],
		},
	},
	{
		Key:  GroupKeyDefectSetup,
		Name: "perm_group_defect_setup",
		Permissions: []Permission{
			PermissionMap["defect_setting:attribute"],
			PermissionMap["defect_setting:field"],
			PermissionMap["defect_setting:status"],
			PermissionMap["defect_setting:priority"],
		},
	},
	{
		Key:  GroupKeyStorage,
		Name: "perm_group_storage",
		Permissions: []Permission{
			PermissionMap["storage:view"],
			PermissionMap["storage:download"],
			PermissionMap["storage:delete"],
		},
	},
	{
		Key:  GroupKeySso,
		Name: "perm_group_sso",
		Permissions: []Permission{
			PermissionMap["sso:setting"],
		},
	},
	{
		Key:         GroupKeyPersonalization,
		Name:        "perm_group_personalization",
		Permissions: []Permission{PermissionMap["personalization:setting"]},
	},
	{
		Key:  GroupKeyAutomation,
		Name: "perm_group_automation",
		Permissions: []Permission{
			PermissionMap["automation:view"],
			PermissionMap["automation:manage"],
		},
	},
	{
		Key:         GroupKeySLA,
		Name:        "perm_group_sla",
		Permissions: []Permission{PermissionMap["sla:view"], PermissionMap["sla:manage"]},
	},
	{
		Key:  GroupKeyApplicationMarket,
		Name: "perm_group_application_market",
		Permissions: []Permission{
			PermissionMap["application_market:view"],
			PermissionMap["application_market:manage"],
		},
	},
	{
		Key:  GroupKeyPipeline,
		Name: "perm_group_pipeline",
		Permissions: []Permission{
			PermissionMap["pipeline:view"],
			PermissionMap["pipeline:manage"],
		},
	},
	{
		Key:  GroupKeyPlanning,
		Name: "perm_group_planning",
		Permissions: []Permission{
			PermissionMap["planning:view"],
			PermissionMap["planning:edit"],
		},
	},
}

// GroupMap provides fast lookup: group key → Group.
var GroupMap = buildGroupMap()

func buildGroupMap() map[string]Group {
	m := make(map[string]Group, len(AllGroups))
	for _, g := range AllGroups {
		m[g.Key] = g
	}
	return m
}

// GetGroup returns the permission group definition by key.
func GetGroup(key string) (Group, bool) {
	g, ok := GroupMap[key]
	return g, ok
}

// GetGroupName returns the i18n display name for a permission group (based on language).
func GetGroupName(key, lang string) string {
	g, ok := GroupMap[key]
	if !ok {
		return key
	}
	return lib_i18n.Show(lang, g.Name)
}

// ======================== Work Item Type Names ========================

// workItemTypeNameKeys maps a work item type key to the i18n key of its built-in name, which
// is the wording embedded in the requirement/defect permission names (e.g. "查看需求").
var workItemTypeNameKeys = map[string]string{
	`requirement`: `workitem_type_requirement`,
	`defect`:      `workitem_type_defect`,
}

// ApplyWorkItemNames replaces the built-in work item type wording in a permission display name
// with the project's configured work item name (e.g. 查看需求 → 查看工单). Types absent from
// names, or configured with the built-in name, keep their original wording.
func ApplyWorkItemNames(text, lang string, names map[string]string) string {
	for typeKey, name := range names {
		key, ok := workItemTypeNameKeys[typeKey]
		if !ok || len(name) == 0 {
			continue
		}
		if builtin := lib_i18n.Show(lang, key); len(builtin) > 0 && builtin != name {
			text = strings.ReplaceAll(text, builtin, name)
		}
	}
	return text
}

// ======================== Role Scope Constants ========================

const (
	ScopeEnterprise = "enterprise" // Enterprise-level role (global across all workspaces).
	ScopeProject    = "project"    // Project-level role (per workspace).
)

// ======================== System Preset Roles ========================

// PresetRole defines a system preset role.
type PresetRole struct {
	Name        string   // i18n key, e.g. "perm_role_admin".
	IsSystem    int      // Whether system preset: 1=yes, 0=no.
	Scope       string   // Role scope: "enterprise" or "project".
	Permissions []string // List of owned permission keys.
}

// PresetRoles contains system preset roles (hardcoded).
// Enterprise roles: owner (users.is_owner, auto full access), admin, member.
// Project roles: admin, member.
var PresetRoles = []PresetRole{
	// ---- Enterprise Roles ----
	{
		Name:     "perm_role_enterprise_admin",
		IsSystem: 1,
		Scope:    ScopeEnterprise,
		Permissions: []string{
			"team_work:view_today", "team_work:view_yesterday", "team_work:view_tomorrow", "team_work:view_custom_date",
			"team_work:view_unscheduled", "team_work:view_scheduled",
			"project:create", "project:manage",
			"plan:view",
			"member:view", "member:invite", "member:remove", "member:edit", "member:reset_password",
			"member:view_role_list",
			"member:create_role", "member:delete_role", "member:edit_role_permissions",
			"storage:view", "storage:download", "storage:delete",
			"sso:setting",
			"personalization:setting",
			"pipeline:view", "pipeline:manage",
			"external_notification:manage",
		},
	},
	{
		Name:     "perm_role_enterprise_member",
		IsSystem: 1,
		Scope:    ScopeEnterprise,
		// Enterprise members hold no enterprise-level permissions by default (pipeline permissions included).
		// If members need pipeline access, the enterprise admin should create a custom enterprise role
		// with pipeline:view (and optionally pipeline:manage) checked, then assign it to those members.
		Permissions: []string{},
	},

	// ---- Project Roles ----
	{
		Name:     "perm_role_project_admin",
		IsSystem: 1,
		Scope:    ScopeProject,
		Permissions: []string{
			"workspace:view", "workspace:edit", "workspace:end",
			"requirement:view", "requirement:create", "requirement:edit", "requirement:delete", "requirement:import", "requirement:export", "requirement:copy", "requirement:manage_system_view", "requirement:manage_category",
			"defect:view", "defect:create", "defect:edit", "defect:delete", "defect:import", "defect:export", "defect:copy", "defect:manage_system_view",
			"gantt:view", "gantt:manage_system_view",
			"version:view", "version:create", "version:edit", "version:delete", "version:manage_system_view",
			"project_info:edit",
			"project_member:add", "project_member:remove", "project_member:change_role",
			"project_role:add", "project_role:delete", "project_role:edit",
			"requirement_setting:field", "requirement_setting:status", "requirement_setting:priority",
			"requirement_setting:attribute",
			"defect_setting:field", "defect_setting:status", "defect_setting:priority",
			"automation:view", "automation:manage",
			"sla:view", "sla:manage",
			"application_market:view", "application_market:manage",
			"defect_setting:attribute",
			"planning:view", "planning:edit",
			"kb:view", "kb:create", "kb:manage",
			"dashboard:view", "dashboard:create",
			"ticket:copy_link", "ticket:manage",
		},
	},
	{
		Name:     "perm_role_project_member",
		IsSystem: 1,
		Scope:    ScopeProject,
		Permissions: []string{
			"workspace:view",
			"requirement:view", "requirement:create", "requirement:edit", "requirement:copy", "requirement:import", "requirement:export",
			"defect:view", "defect:create", "defect:edit", "defect:copy", "defect:import", "defect:export",
			"gantt:view",
			"version:view", "version:create", "version:edit",
			"kb:view", "kb:create",
			"dashboard:view",
			// Planning is not open to regular members by default; enable it via role management when needed.
		},
	},
}

// GetPresetRoleName returns the i18n display name for a preset role.
func GetPresetRoleName(nameKey, lang string) string {
	return lib_i18n.Show(lang, nameKey)
}
