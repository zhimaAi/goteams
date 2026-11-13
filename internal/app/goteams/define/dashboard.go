// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// Project dashboard table names.
const (
	TableWorkspaceDashboards        = `workspace_dashboards`
	TableDashboardVisibilityRoles   = `dashboard_visibility_roles`
	TableDashboardVisibilityMembers = `dashboard_visibility_members`
	TableDashboardEditors           = `dashboard_editors`
	TableDashboardCards             = `dashboard_cards`
	TableDashboardAnnouncements     = `dashboard_announcements`
	TableDashboardUserStates        = `dashboard_user_states`
)

// Dashboard permission keys.
const (
	PermissionDashboardView   = `dashboard:view`
	PermissionDashboardCreate = `dashboard:create`
)

// Dashboard visibility types.
const (
	DashboardVisibilityPublic  = `public`
	DashboardVisibilitySelf    = `self`
	DashboardVisibilityRoles   = `roles`
	DashboardVisibilityMembers = `members`
)

// Dashboard card types supported in the first release.
const (
	DashboardCardTypeMetric       = `metric`
	DashboardCardTypeChart        = `chart`
	DashboardCardTypeProjectInfo  = `project_info`
	DashboardCardTypeAnnouncement = `announcement`
	DashboardCardTypeText         = `text`
)

// Dashboard rich-text content modes.
const (
	DashboardContentModeRichText = `rich-text`
)

// Dashboard metric data scopes.
const (
	DashboardMetricDataScopeAll         = `all`
	DashboardMetricDataScopeRequirement = `requirement`
	DashboardMetricDataScopeDefect      = `defect`
)

// DashboardMetricSLAAchievementRate 按所选阶段计算严格口径的工单SLA达成百分比。
const (
	DashboardMetricSLAAchievementRate = "sla_achievement_rate"
	// DashboardMetricSLAStageAchievementRate 使用指标选项绑定的单个阶段。
	DashboardMetricSLAStageAchievementRate = "sla_stage_achievement_rate"
	// DashboardMetricSLAStageCriticalCount 统计所选阶段中进入临界阈值的工单。
	DashboardMetricSLAStageCriticalCount = "sla_stage_critical_count"
	// DashboardMetricSLAFirstResponseRate 按单个首响阶段计算达成百分比。
	DashboardMetricSLAFirstResponseRate = "sla_first_response_rate"
	// DashboardMetricSLAResolutionRate 按单个解决阶段计算达成百分比。
	DashboardMetricSLAResolutionRate = "sla_resolution_rate"
	// DashboardMetricSLAOverdueCount 按工单去重统计存在任一超时阶段的数量。
	DashboardMetricSLAOverdueCount = "sla_overdue_count"
	// DashboardMetricSLAOverdueRate 以已进入且可计算的工单为分母计算超时百分比。
	DashboardMetricSLAOverdueRate = "sla_overdue_rate"
)

// Dashboard metric type keys.
const (
	DashboardMetricWorkItemCount             = `work_item_count`
	DashboardMetricCreatedRequirementCount   = `created_requirement_count`
	DashboardMetricCreatedDefectCount        = `created_defect_count`
	DashboardMetricCompletedRequirementCount = `completed_requirement_count`
	DashboardMetricCompletedDefectCount      = `completed_defect_count`
	DashboardMetricOverdueRequirementCount   = `overdue_requirement_count`
	DashboardMetricUnresolvedDefectCount     = `unresolved_defect_count`
	// DashboardMetricClosedCount counts closed work items.
	DashboardMetricClosedCount = "closed_count"
	// DashboardMetricInProgressCount counts work items between initial and terminal statuses.
	DashboardMetricInProgressCount = "in_progress_count"
	// DashboardMetricIssuePendingCount counts work items awaiting completion.
	DashboardMetricIssuePendingCount = "issue_pending_count"
	// DashboardMetricIssueCreatedToday counts work items created today.
	DashboardMetricIssueCreatedToday = "issue_created_today"
	// DashboardMetricIssueCompletedToday counts work items completed today.
	DashboardMetricIssueCompletedToday = "issue_completed_today"
	// DashboardMetricIssueSLADueSoon counts work items with an approaching SLA deadline.
	DashboardMetricIssueSLADueSoon    = "issue_sla_due_soon"
	DashboardMetricEstimatedWorkHours = `estimated_work_hours`
	DashboardMetricCompletedWorkHours = `completed_work_hours`
	DashboardMetricRemainingWorkHours = `remaining_work_hours`
	DashboardMetricOvertimeWorkHours  = `overtime_work_hours`
)

// Dashboard layout and payload limits.
const (
	DashboardGridColumns          = 12
	DashboardGridMaxRows          = 1000000
	DashboardMaxCardsPerDashboard = 100
	DashboardNameMaxLength        = 100
	DashboardCardTitleMaxLength   = 100
	DashboardContentMaxBytes      = 10 * 1024 * 1024
	DashboardConfigMaxBytes       = 64 * 1024
	DashboardFiltersMaxBytes      = 64 * 1024
)

// Dashboard source types stored in the enterprise-file ledger.
const (
	SourceTypeDashboardCard = `dashboard_card`
	// Enterprise file ledgers limit source_type to 20 characters.
	SourceTypeDashboardAnnouncement = `dashboard_announce`
)

// Dashboard message keys.
const (
	MsgDashboardNotFound           = `dashboard_not_found`
	MsgDashboardForbidden          = `dashboard_forbidden`
	MsgDashboardRevisionConflict   = `dashboard_revision_conflict`
	MsgDashboardFeatureDisabled    = `dashboard_feature_disabled`
	MsgDashboardAnnouncementExists = `dashboard_announcement_exists`
	MsgDashboardInvalidLayout      = `dashboard_invalid_layout`
)

// DashboardFilters extends the shared three-section filter tree with work item types.
// Empty WorkItemTypes includes both requirements and defects.
type DashboardFilters struct {
	GanttViewFilters
	WorkItemTypes []string `json:"work_item_types,omitempty"`
}
