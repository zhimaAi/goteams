// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"
)

var (
	// ErrDashboardNameInvalid indicates a dashboard name is blank or exceeds its column limit.
	ErrDashboardNameInvalid = errors.New(`dashboard name invalid`)
	// ErrDashboardCardTitleInvalid indicates a card title exceeds its column limit.
	ErrDashboardCardTitleInvalid = errors.New(`dashboard card title invalid`)
	// ErrDashboardCardTypeInvalid indicates an unsupported first-release card type.
	ErrDashboardCardTypeInvalid = errors.New(`dashboard card type invalid`)
	// ErrDashboardContentModeInvalid indicates an unsupported rich-text content mode.
	ErrDashboardContentModeInvalid = errors.New(`dashboard content mode invalid`)
	// ErrDashboardContentInvalid indicates a content payload exceeds the supported size.
	ErrDashboardContentInvalid = errors.New(`dashboard content invalid`)
	// ErrDashboardConfigInvalid indicates a card config is not a bounded JSON object.
	ErrDashboardConfigInvalid = errors.New(`dashboard config invalid`)
	// ErrDashboardFiltersInvalid indicates a combined requirement/defect filter payload is invalid.
	ErrDashboardFiltersInvalid = errors.New(`dashboard filters invalid`)
	// ErrDashboardVisibilityInvalid indicates a per-dashboard visibility payload is invalid.
	ErrDashboardVisibilityInvalid = errors.New(`dashboard visibility invalid`)
	// ErrDashboardLayoutInvalid indicates a grid layout is malformed, out of bounds, or overlapping.
	ErrDashboardLayoutInvalid = errors.New(`dashboard layout invalid`)
	// ErrDashboardOrderInvalid indicates a complete dashboard order is malformed.
	ErrDashboardOrderInvalid = errors.New(`dashboard order invalid`)
	// ErrDashboardRevisionInvalid indicates optimistic concurrency metadata is absent or invalid.
	ErrDashboardRevisionInvalid = errors.New(`dashboard revision invalid`)
	// ErrDashboardMetricInvalid indicates an incompatible metric type/data-scope pair.
	ErrDashboardMetricInvalid = errors.New(`dashboard metric invalid`)
	// ErrDashboardFileIDsInvalid indicates a duplicate or non-positive uploaded file reference.
	ErrDashboardFileIDsInvalid = errors.New(`dashboard file ids invalid`)
)

// DashboardLayoutItem is the validate-layer representation of one 12-column grid placement.
type DashboardLayoutItem struct {
	Id int
	X  int
	Y  int
	W  int
	H  int
}

// DashboardVisibility validates the configured visible roles/members and additional editors.
func DashboardVisibility(visibilityType string, roleIds, memberIds, editorIds []int) error {
	if err := dashboardPositiveUniqueIDs(roleIds); err != nil {
		return ErrDashboardVisibilityInvalid
	}
	if err := dashboardPositiveUniqueIDs(memberIds); err != nil {
		return ErrDashboardVisibilityInvalid
	}
	if err := dashboardPositiveUniqueIDs(editorIds); err != nil {
		return ErrDashboardVisibilityInvalid
	}
	switch visibilityType {
	case define.DashboardVisibilityPublic, define.DashboardVisibilitySelf:
		if len(roleIds) > 0 || len(memberIds) > 0 {
			return ErrDashboardVisibilityInvalid
		}
	case define.DashboardVisibilityRoles:
		if len(roleIds) == 0 || len(memberIds) > 0 {
			return ErrDashboardVisibilityInvalid
		}
	case define.DashboardVisibilityMembers:
		if len(memberIds) == 0 || len(roleIds) > 0 {
			return ErrDashboardVisibilityInvalid
		}
	default:
		return ErrDashboardVisibilityInvalid
	}
	return nil
}

// DashboardName validates a user-entered dashboard name after surrounding whitespace is ignored.
func DashboardName(name string) error {
	length := utf8.RuneCountInString(strings.TrimSpace(name))
	if length < 1 || length > define.DashboardNameMaxLength {
		return ErrDashboardNameInvalid
	}
	return nil
}

// DashboardCardTitle validates an optional display title for a card.
func DashboardCardTitle(title string) error {
	if utf8.RuneCountInString(title) > define.DashboardCardTitleMaxLength {
		return ErrDashboardCardTitleInvalid
	}
	return nil
}

// DashboardCardType validates one of the first-release dashboard card types.
func DashboardCardType(cardType string) error {
	switch cardType {
	case define.DashboardCardTypeChart, define.DashboardCardTypeMetric, define.DashboardCardTypeProjectInfo,
		define.DashboardCardTypeAnnouncement, define.DashboardCardTypeText:
		return nil
	default:
		return ErrDashboardCardTypeInvalid
	}
}

// DashboardContentMode validates an optional content mode. Cards without editable content leave it empty.
func DashboardContentMode(contentMode string) error {
	if contentMode == `` || contentMode == define.DashboardContentModeRichText {
		return nil
	}
	return ErrDashboardContentModeInvalid
}

// DashboardContent validates a rich-text payload size in bytes.
func DashboardContent(content string) error {
	if len([]byte(content)) > define.DashboardContentMaxBytes {
		return ErrDashboardContentInvalid
	}
	return nil
}

// DashboardConfig validates an optional card configuration object without normalizing its JSON.
func DashboardConfig(config []byte) error {
	config = bytes.TrimSpace(config)
	if len(config) == 0 {
		return nil
	}
	if len(config) > define.DashboardConfigMaxBytes || !json.Valid(config) || config[0] != '{' {
		return ErrDashboardConfigInvalid
	}
	return nil
}

// DashboardFilters validates the combined {common,requirement,defect} filter payload.
// Empty and JSON null both mean that no additional filter is applied.
func DashboardFilters(raw []byte, dataScope string) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte(`null`)) {
		return nil
	}
	if len(raw) > define.DashboardFiltersMaxBytes || !json.Valid(raw) || raw[0] != '{' {
		return ErrDashboardFiltersInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var sections define.DashboardFilters
	if err := decoder.Decode(&sections); err != nil {
		return ErrDashboardFiltersInvalid
	}
	if len(sections.WorkItemTypes) > 2 {
		return ErrDashboardFiltersInvalid
	}
	seenTypes := make(map[string]bool, len(sections.WorkItemTypes))
	for _, itemType := range sections.WorkItemTypes {
		if (itemType != define.TypeKeyRequirement && itemType != define.TypeKeyDefect) || seenTypes[itemType] {
			return ErrDashboardFiltersInvalid
		}
		seenTypes[itemType] = true
	}
	if sections.Common != nil {
		if err := ValidateViewFilters(*sections.Common); err != nil {
			return ErrDashboardFiltersInvalid
		}
		if err := ValidateGanttCommonFilterFields(*sections.Common); err != nil {
			return ErrDashboardFiltersInvalid
		}
	}
	if sections.Requirement != nil {
		if err := ValidateViewFiltersForType(*sections.Requirement, define.TypeKeyRequirement); err != nil {
			return ErrDashboardFiltersInvalid
		}
	}
	if sections.Defect != nil {
		if err := ValidateViewFiltersForType(*sections.Defect, define.TypeKeyDefect); err != nil {
			return ErrDashboardFiltersInvalid
		}
	}
	switch dataScope {
	case define.DashboardMetricDataScopeAll:
	case define.DashboardMetricDataScopeRequirement:
		if sections.Defect != nil {
			return ErrDashboardFiltersInvalid
		}
	case define.DashboardMetricDataScopeDefect:
		if sections.Requirement != nil {
			return ErrDashboardFiltersInvalid
		}
	default:
		return ErrDashboardFiltersInvalid
	}
	return nil
}

// DashboardLayout validates one complete batch of card placements in the 12-column dashboard grid.
func DashboardLayout(items []DashboardLayoutItem) error {
	if len(items) > define.DashboardMaxCardsPerDashboard {
		return ErrDashboardLayoutInvalid
	}
	seen := make(map[int]struct{}, len(items))
	for _, item := range items {
		if item.Id <= 0 || item.X < 0 || item.Y < 0 || item.W < 1 || item.H < 1 ||
			item.W > define.DashboardGridColumns || item.H > define.DashboardGridMaxRows ||
			item.X > define.DashboardGridColumns-item.W || item.Y > define.DashboardGridMaxRows-item.H {
			return ErrDashboardLayoutInvalid
		}
		if _, duplicate := seen[item.Id]; duplicate {
			return ErrDashboardLayoutInvalid
		}
		seen[item.Id] = struct{}{}
	}
	for i := range items {
		for j := i + 1; j < len(items); j++ {
			if dashboardLayoutsOverlap(items[i], items[j]) {
				return ErrDashboardLayoutInvalid
			}
		}
	}
	return nil
}

// DashboardOrder validates a deduplicated positive dashboard ID sequence.
func DashboardOrder(ids []int) error {
	if len(ids) == 0 || dashboardPositiveUniqueIDs(ids) != nil {
		return ErrDashboardOrderInvalid
	}
	return nil
}

// DashboardRevision validates a positive optimistic-lock revision.
func DashboardRevision(revision int64) error {
	if revision <= 0 {
		return ErrDashboardRevisionInvalid
	}
	return nil
}

// DashboardMetric validates the metric type/data-scope compatibility that does not require database state.
func DashboardMetric(metricType, dataScope string) error {
	switch metricType {
	case define.DashboardMetricWorkItemCount, define.DashboardMetricClosedCount, define.DashboardMetricInProgressCount:
		if dataScope == define.DashboardMetricDataScopeAll || dataScope == define.DashboardMetricDataScopeRequirement || dataScope == define.DashboardMetricDataScopeDefect {
			return nil
		}
	case define.DashboardMetricSLAAchievementRate, define.DashboardMetricSLAStageAchievementRate, define.DashboardMetricSLAStageCriticalCount,
		define.DashboardMetricSLAFirstResponseRate, define.DashboardMetricSLAResolutionRate, define.DashboardMetricSLAOverdueCount, define.DashboardMetricSLAOverdueRate, define.DashboardMetricIssuePendingCount, define.DashboardMetricIssueCreatedToday,
		define.DashboardMetricIssueCompletedToday, define.DashboardMetricIssueSLADueSoon,
		define.DashboardMetricCreatedRequirementCount, define.DashboardMetricCompletedRequirementCount,
		define.DashboardMetricOverdueRequirementCount,
		define.DashboardMetricEstimatedWorkHours, define.DashboardMetricCompletedWorkHours,
		define.DashboardMetricRemainingWorkHours, define.DashboardMetricOvertimeWorkHours:
		if dataScope == define.DashboardMetricDataScopeRequirement {
			return nil
		}
	case define.DashboardMetricCreatedDefectCount, define.DashboardMetricCompletedDefectCount,
		define.DashboardMetricUnresolvedDefectCount:
		if dataScope == define.DashboardMetricDataScopeDefect {
			return nil
		}
	}
	return ErrDashboardMetricInvalid
}

// DashboardFileIDs validates the optional set of newly uploaded rich-text assets.
func DashboardFileIDs(fileIds []int) error {
	if dashboardPositiveUniqueIDs(fileIds) != nil {
		return ErrDashboardFileIDsInvalid
	}
	return nil
}

func dashboardPositiveUniqueIDs(ids []int) error {
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return errors.New(`id must be positive`)
		}
		if _, duplicate := seen[id]; duplicate {
			return errors.New(`id must be unique`)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func dashboardLayoutsOverlap(a, b DashboardLayoutItem) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

var dashboardMetricColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// DashboardMetricStyle validates the optional display unit and CSS hex color.
func DashboardMetricStyle(unit, color string) error {
	if utf8.RuneCountInString(unit) > 20 || (color != "" && !dashboardMetricColorPattern.MatchString(color)) {
		return ErrDashboardMetricInvalid
	}
	return nil
}

// DashboardMetricRecentDays validates the optional rolling "recent N days" window; 0 means no limit.
func DashboardMetricRecentDays(days int) error {
	if days < 0 || days > 365 {
		return ErrDashboardMetricInvalid
	}
	return nil
}

// DashboardChart validates the bounded statistical chart configuration.
func DashboardChart(kind, scope, order, palette, requirementColor, defectColor string, from, to int, statuses, priorities, custom []int) error {
	if kind != "status_bar" && kind != "priority_bar" && kind != "completion_line" && kind != "status_pie" &&
		kind != "daily_created_line" && kind != "assignee_rank_bar" && kind != "iteration_burndown" && kind != "category_stats" && kind != "workload_stats" {
		return ErrDashboardCardTypeInvalid
	}
	// 空字符串表示需求与缺陷都未勾选，统计卡不取数并展示空态。
	if scope != "" && scope != "all" && scope != "requirement" && scope != "defect" {
		return ErrDashboardMetricInvalid
	}
	if order != "default" && order != "asc" && order != "desc" && order != "custom" {
		return ErrDashboardMetricInvalid
	}
	if palette != "status" && palette != "bright" && palette != "soft" && palette != "cool" {
		return ErrDashboardMetricInvalid
	}
	if DashboardMetricStyle("", requirementColor) != nil || DashboardMetricStyle("", defectColor) != nil || requirementColor == "" || defectColor == "" {
		return ErrDashboardMetricInvalid
	}
	if from < to || to < 0 || from > 3650 || from-to > 365 {
		return ErrDashboardMetricInvalid
	}
	for _, ids := range [][]int{statuses, priorities, custom} {
		if len(ids) > 500 {
			return ErrDashboardMetricInvalid
		}
		seen := map[int]bool{}
		for _, id := range ids {
			if id < 0 || seen[id] {
				return ErrDashboardMetricInvalid
			}
			seen[id] = true
		}
	}
	return nil
}

// DashboardSLATrend validates the SLA chart's date range, optional target and series colors.
func DashboardSLATrend(scope string, days int, target *float64, rateColor, overdueColor string) error {
	if scope != "requirement" || days < 1 || days > 365 || (target != nil && (*target < 0 || *target > 100)) {
		return ErrDashboardMetricInvalid
	}
	return DashboardChartColors(rateColor, overdueColor)
}

// DashboardSLADimension validates the group selection and required stage cardinality.
func DashboardSLADimension(dimension, scope string, priorities, categories []int) error {
	if scope != "requirement" || (dimension != "priority" && dimension != "category") {
		return ErrDashboardMetricInvalid
	}
	for _, ids := range [][]int{priorities, categories} {
		if len(ids) > 500 {
			return ErrDashboardMetricInvalid
		}
		seen := make(map[int]bool, len(ids))
		for _, id := range ids {
			if id < 0 || seen[id] {
				return ErrDashboardMetricInvalid
			}
			seen[id] = true
		}
	}
	return nil
}

// DashboardSLARiskFilter 校验临时筛选和分页边界。
func DashboardSLARiskFilter(from, to int64, assigneeId int, priorityId *int, page, size int) error {
	if page < 1 || page > 1000000 || size < 1 || size > 100 ||
		assigneeId < 0 || (priorityId != nil && *priorityId < 0) {
		return ErrDashboardMetricInvalid
	}
	if (from != 0 || to != 0) && (from <= 0 || to <= from || to-from > 367*86400) {
		return ErrDashboardMetricInvalid
	}
	return nil
}

// DashboardChartColors validates a group of non-empty CSS hex colors.
func DashboardChartColors(colors ...string) error {
	for _, color := range colors {
		if color == "" || DashboardMetricStyle("", color) != nil {
			return ErrDashboardMetricInvalid
		}
	}
	return nil
}

// DashboardChartMetricType validates the aggregate metric of the ranking chart.
func DashboardChartMetricType(metricType string) error {
	if metricType != "work_item_count" {
		return ErrDashboardMetricInvalid
	}
	return nil
}

// DashboardCategoryScope 校验分类统计的处理人范围。
func DashboardCategoryScope(scope string, ids []int) error {
	switch scope {
	case "", "all", "self":
		if len(ids) == 0 {
			return nil
		}
	case "roles", "members":
		if len(ids) > 0 && len(ids) <= 500 && dashboardPositiveUniqueIDs(ids) == nil {
			return nil
		}
	}
	return ErrDashboardMetricInvalid
}

// DashboardSLAMetricOptions 校验阶段、时间范围、环比和曲线颜色，不访问数据库。
func DashboardSLAMetricOptions(metricType string, ids []int, comparison bool, lineColor string, slaRecentDays *int) error {
	if !strings.HasPrefix(metricType, "sla_") {
		if len(ids) > 0 || comparison || lineColor != "" || slaRecentDays != nil {
			return ErrDashboardMetricInvalid
		}
		return nil
	}
	if DashboardMetricStyle("", lineColor) != nil {
		return ErrDashboardMetricInvalid
	}
	if slaRecentDays != nil && (DashboardMetricRecentDays(*slaRecentDays) != nil || (comparison && *slaRecentDays == 0)) {
		return ErrDashboardMetricInvalid
	}
	switch metricType {
	case define.DashboardMetricSLAOverdueCount, define.DashboardMetricSLAOverdueRate:
		// 旧卡片保存了阶段范围，读取时按任意阶段计算。
		return nil
	case define.DashboardMetricSLAStageCriticalCount:
		// 工作空间阶段不足时，模板仍可保留待配置的指标卡。
		if len(ids) <= 1 && dashboardPositiveUniqueIDs(ids) == nil {
			return nil
		}
	case define.DashboardMetricSLAStageAchievementRate, define.DashboardMetricSLAFirstResponseRate, define.DashboardMetricSLAResolutionRate:
		if len(ids) == 1 && dashboardPositiveUniqueIDs(ids) == nil {
			return nil
		}
	case define.DashboardMetricSLAAchievementRate:
		if len(ids) > 0 && dashboardPositiveUniqueIDs(ids) == nil {
			return nil
		}
	}
	return ErrDashboardMetricInvalid
}
