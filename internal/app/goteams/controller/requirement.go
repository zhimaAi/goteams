// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ======================== Requirement CRUD ========================

// customFieldValues describes a dynamic value set keyed by workspace custom field ID.
type customFieldValues map[string]any

// listRequirementsQuery are the query params for the requirement/defect list API;
// filters are bound as strings first to distinguish "not passed" from "invalid value passed".
type listRequirementsQuery struct {
	Type                 string `form:"type"`
	CategoryId           string `form:"category_id"`
	CategoryExact        string `form:"category_exact"`
	StatusId             string `form:"status_id"`
	StatusIds            string `form:"status_ids"`
	Priority             string `form:"priority"`
	PriorityIds          string `form:"priority_ids"`
	AssigneeIds          string `form:"assignee_ids"`
	AssigneeId           string `form:"assignee_id"`
	VersionId            string `form:"version_id"`
	VersionIds           string `form:"version_ids"`
	CreatedBy            string `form:"created_by"`
	CreatedByIds         string `form:"created_by_ids"`
	ParentId             string `form:"parent_id"`
	RootOnly             string `form:"root_only"`
	Search               string `form:"search"`
	TitleSearch          string `form:"title_search"`
	ExcludeDescendantsOf string `form:"exclude_descendants_of"`
	PlanningId           string `form:"planning_id"`
	ViewId               string `form:"view_id"`
	SearchFilters        string `form:"search_filters"`
	Timezone             string `form:"timezone"`
	// Pagination params are also bound as strings: invalid values fall back to default page/limit without erroring the frontend.
	Page          string `form:"page"`
	Size          string `form:"size"`
	SortField     string `form:"sort_field"`
	SortDirection string `form:"sort_direction"`
}

// listRequirementsResponseItem is the response struct for a requirement/defect list item;
// numeric fields keep the existing contract (output as strings), and the assignee field is an array.
type listRequirementsResponseItem struct {
	Id                 string   `json:"id"`
	WorkspaceId        string   `json:"workspace_id"`
	ParentId           string   `json:"parent_id"`
	CategoryId         string   `json:"category_id"`
	Title              string   `json:"title"`
	Type               string   `json:"type"`
	Priority           string   `json:"priority"`
	StatusId           string   `json:"status_id"`
	TemplateId         string   `json:"template_id"`
	TemplateName       string   `json:"template_name"`
	CreatedBy          string   `json:"created_by"`
	VersionId          string   `json:"version_id"`
	PlannedStartDate   string   `json:"planned_start_date"`
	PlannedEndDate     string   `json:"planned_end_date"`
	EstimatedWorkHours string   `json:"estimated_work_hours"`
	CompletedWorkHours string   `json:"completed_work_hours"`
	RemainingWorkHours string   `json:"remaining_work_hours"`
	OvertimeWorkHours  string   `json:"overtime_work_hours"`
	WorkProgress       int      `json:"work_progress"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
	StatusName         string   `json:"status_name"`
	StatusColor        string   `json:"status_color"`
	AgingStatus        int      `json:"aging_status"`
	PriorityName       string   `json:"priority_name"`
	PriorityColor      string   `json:"priority_color"`
	CreatorName        string   `json:"creator_name"`
	WorkspaceName      string   `json:"workspace_name"`
	VersionName        string   `json:"version_name"`
	ParentTitle        string   `json:"parent_title"`
	AssigneeIds        []int    `json:"assignee_ids"`
	AssigneeNames      []string `json:"assignee_names"`
	AssigneeAvatars    []string `json:"assignee_avatars"`
	AssigneeUsernames  []string `json:"assignee_usernames"`
	HasChildren        bool     `json:"has_children"`
	CustomFields       string   `json:"custom_fields"`
}

// listRequirementsResponse is the paginated response for the requirement/defect list API.
type listRequirementsResponse struct {
	List  []listRequirementsResponseItem `json:"list"`
	Total int                            `json:"total"`
	Page  int                            `json:"page"`
	Size  int                            `json:"size"`
}

// toListRequirementsResponse converts the business model to the response; status/priority names are stored as i18n keys, translated here by request language.
func toListRequirementsResponse(model business.ListRequirementsModel, lang string) (listRequirementsResponse, error) {
	items := make([]listRequirementsResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		customFields, err := tool.JsonEncode(item.CustomFields)
		if err != nil {
			return listRequirementsResponse{}, err
		}
		items = append(items, listRequirementsResponseItem{
			Id:                 strconv.Itoa(item.Id),
			WorkspaceId:        strconv.Itoa(item.WorkspaceId),
			ParentId:           strconv.Itoa(item.ParentId),
			CategoryId:         strconv.Itoa(item.CategoryId),
			Title:              item.Title,
			Type:               item.Type,
			Priority:           strconv.Itoa(item.Priority),
			StatusId:           strconv.Itoa(item.StatusId),
			TemplateId:         strconv.Itoa(item.TemplateId),
			TemplateName:       item.TemplateName,
			CreatedBy:          strconv.Itoa(item.CreatedBy),
			VersionId:          strconv.Itoa(item.VersionId),
			PlannedStartDate:   strconv.FormatInt(item.PlannedStartDate, 10),
			PlannedEndDate:     strconv.FormatInt(item.PlannedEndDate, 10),
			EstimatedWorkHours: formatRequirementWorkHours(item.EstimatedWorkHours),
			CompletedWorkHours: formatRequirementWorkHours(item.CompletedWorkHours),
			RemainingWorkHours: formatRequirementWorkHours(item.RemainingWorkHours),
			OvertimeWorkHours:  formatRequirementWorkHours(item.OvertimeWorkHours),
			WorkProgress:       item.WorkProgress,
			CreatedAt:          strconv.FormatInt(item.CreatedAt, 10),
			UpdatedAt:          strconv.FormatInt(item.UpdatedAt, 10),
			StatusName:         lib_i18n.ShowStored(lang, item.StatusName),
			StatusColor:        item.StatusColor,
			AgingStatus:        item.AgingStatus,
			PriorityName:       lib_i18n.ShowStored(lang, item.PriorityName),
			PriorityColor:      item.PriorityColor,
			CreatorName:        item.CreatorName,
			WorkspaceName:      item.WorkspaceName,
			VersionName:        item.VersionName,
			ParentTitle:        item.ParentTitle,
			AssigneeIds:        item.AssigneeIds,
			AssigneeNames:      item.AssigneeNames,
			AssigneeAvatars:    item.AssigneeAvatars,
			AssigneeUsernames:  item.AssigneeUsernames,
			HasChildren:        item.HasChildren,
			CustomFields:       customFields,
		})
	}
	return listRequirementsResponse{List: items, Total: model.Total, Page: model.Page, Size: model.Size}, nil
}

// requirementAgingNodeResponse is one row of the requirement aging detail.
type requirementAgingNodeResponse struct {
	IsTotal      bool   `json:"is_total"`
	StatusName   string `json:"status_name"`
	StatusColor  string `json:"status_color"`
	EnterTime    int64  `json:"enter_time"`
	Handler      string `json:"handler"`
	DurationSec  int64  `json:"duration_sec"`
	DurationText string `json:"duration_text"`
	OverdueState string `json:"overdue_state"`
	Tooltip      string `json:"tooltip"`
}

type requirementAgingResponse struct {
	Nodes []requirementAgingNodeResponse `json:"nodes"`
}

// GetRequirementAging returns the per-status residence detail and overall aging conclusion.
func GetRequirementAging(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.aging.workspace_id`, err)
		return
	}
	adminId := cast.ToInt(c.GetInt(`adminId`))
	reqIdStr := c.Param(`reqId`)
	reqId := cast.ToInt(reqIdStr)
	if reqId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.aging.id`, nil)
		return
	}
	settings, err := business.GetWorkspaceSettings(business.WorkspaceSettingsModel{AdminId: adminId, WsId: cast.ToString(wsId)})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.aging.settings`, err)
		return
	}
	if settings.SLAActivatedAt > 0 {
		abortConflict(c, lib_i18n.Show(lang, define.MsgSLAConflict))
		return
	}
	nodes, err := business.GetRequirementAging(business.GetRequirementAgingModel{
		AdminId:        adminId,
		WorkspaceId:    cast.ToString(wsId),
		RequirementId:  reqIdStr,
		Form:           settings.RequirementAgingControl,
		FinalStatusIds: settings.RequirementFinalStatusIds,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.aging`, err)
		return
	}
	resp := make([]requirementAgingNodeResponse, 0, len(nodes))
	for _, n := range nodes {
		statusName := ``
		if n.StatusKey != `` {
			statusName = lib_i18n.ShowStored(lang, n.StatusKey)
		}
		resp = append(resp, requirementAgingNodeResponse{
			IsTotal:      n.IsTotal,
			StatusName:   statusName,
			StatusColor:  n.StatusColor,
			EnterTime:    n.EnterTime,
			Handler:      n.Handler,
			DurationSec:  n.DurationSec,
			DurationText: n.DurationText,
			OverdueState: n.OverdueState,
			Tooltip:      n.Tooltip,
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    requirementAgingResponse{Nodes: resp},
	})
}

type requirementAssigneeIDs []int

func (ids *requirementAssigneeIDs) UnmarshalJSON(data []byte) error {
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	result := make([]int, 0, len(values))
	for _, value := range values {
		var id int
		if err := json.Unmarshal(value, &id); err == nil {
			result = append(result, id)
			continue
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return err
		}
		id, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			return err
		}
		result = append(result, id)
	}
	*ids = result
	return nil
}

// ListRequirements returns the requirement list (supports pagination and filtering).
func ListRequirements(c *gin.Context) {
	listRequirements(c, ``)
}

// ListGanttRequirements 按甘特图权限读取工作项，查询和响应契约与普通列表共用。
func ListGanttRequirements(c *gin.Context) {
	listRequirements(c, define.TypeKeyGantt)
}

func listRequirements(c *gin.Context, contextType string) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.workspace_id`, err)
		return
	}
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))
	ganttContext := contextType == define.TypeKeyGantt
	if ganttContext {
		enabled, appErr := business.IsWorkspaceApplicationEnabled(adminId, wsId, define.AppKeyGantt)
		if appErr != nil {
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `gantt.list.application`, appErr)
			return
		}
		if !enabled {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
	}
	var query listRequirementsQuery
	if err = c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.bind`, err)
		return
	}
	if len(query.Type) > 0 {
		if err = validate.ValidateRequirementType(query.Type); err != nil {
			abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `requirement.list.type`, err)
			return
		}
		if !ganttContext && !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), query.Type, `view`) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
	}
	if len(query.RootOnly) > 0 && query.RootOnly != `0` && query.RootOnly != `1` {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.root_only`, errors.New(`root_only must be 0 or 1`))
		return
	}
	if query.SortField != `` && !business.IsRequirementSortField(query.SortField) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.sort_field`, errors.New(`unsupported sort_field`))
		return
	}
	if query.SortDirection != `` && query.SortDirection != `asc` && query.SortDirection != `desc` {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.sort_direction`, errors.New(`sort_direction must be asc or desc`))
		return
	}
	if len(query.CategoryExact) > 0 && query.CategoryExact != `0` && query.CategoryExact != `1` {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.category_exact`, errors.New(`category_exact must be 0 or 1`))
		return
	}
	if query.CategoryExact == `1` && query.CategoryId == `` {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.category_exact`, errors.New(`category_exact requires category_id`))
		return
	}
	// parent_id and root_only jointly decide "filter by parent": explicit parent_id wins; root_only=1 equals fetching only root nodes.
	var parentId *int
	if len(query.ParentId) > 0 {
		value, parseErr := strconv.Atoi(query.ParentId)
		if parseErr != nil || value < 0 {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.parent_id`, errors.New(`parent_id must be a non-negative integer`))
			return
		}
		parentId = &value
	} else if query.RootOnly == `1` {
		root := 0
		parentId = &root
	}
	if len(query.ExcludeDescendantsOf) > 0 {
		value, parseErr := strconv.Atoi(query.ExcludeDescendantsOf)
		if parseErr != nil || value <= 0 {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.exclude_descendants_of`, errors.New(`exclude_descendants_of must be a positive integer`))
			return
		}
	}
	assigneeIdsText := query.AssigneeIds
	if assigneeIdsText == `` {
		assigneeIdsText = query.AssigneeId
	}
	if err = validatePositiveIDList(assigneeIdsText); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.assignee_ids`, err)
		return
	}
	statusIds, err := parseIntegerIDList(query.StatusIds, false)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.status_ids`, err)
		return
	}
	priorityIds, err := parseIntegerIDList(query.PriorityIds, true)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.priority_ids`, err)
		return
	}
	versionIds, err := parseIntegerIDList(query.VersionIds, true)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.version_ids`, err)
		return
	}
	createdByIds, err := parseIntegerIDList(query.CreatedByIds, false)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.created_by_ids`, err)
		return
	}

	itemType := query.Type
	if itemType == `` && !ganttContext {
		canViewRequirement := hasWorkItemPermission(adminId, userId, cast.ToString(wsId), define.TypeKeyRequirement, `view`)
		canViewDefect := hasWorkItemPermission(adminId, userId, cast.ToString(wsId), define.TypeKeyDefect, `view`)
		if canViewRequirement && !canViewDefect {
			itemType = define.TypeKeyRequirement
		}
		if !canViewRequirement && canViewDefect {
			itemType = define.TypeKeyDefect
		}
	}

	searchFilters, parseSearchErr := parseAndValidateSearchFilters(query.SearchFilters, adminId, wsId, userId, itemType)
	if parseSearchErr != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.search_filters`, parseSearchErr)
		return
	}
	timezone, timezoneErr := normalizeRequestTimezone(query.Timezone)
	if timezoneErr != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.timezone`, timezoneErr)
		return
	}

	var categoryIds []int
	if query.CategoryId != `` {
		if itemType != define.TypeKeyRequirement {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.category_type`, errors.New(`category_id requires type=requirement`))
			return
		}
		categoryId, parseErr := strconv.Atoi(query.CategoryId)
		if parseErr != nil || categoryId < 0 {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.list.category_id`, errors.New(`category_id must be a non-negative integer`))
			return
		}
		if categoryId == 0 || query.CategoryExact == `1` {
			categoryIds = []int{categoryId}
		} else {
			categoryIds, err = business.ListRequirementCategorySubtreeIDs(adminId, wsId, categoryId)
			if errors.Is(err, business.ErrRequirementCategoryNotFound) {
				categoryIds = []int{0}
			} else if err != nil {
				abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.list.category_tree`, err)
				return
			}
		}
	}

	result, err := business.ListRequirements(business.ListRequirementsModel{
		AdminId:              adminId,
		WorkspaceId:          wsId,
		UserId:               userId,
		StatusId:             cast.ToInt(query.StatusId),
		StatusIds:            statusIds,
		Priority:             cast.ToInt(query.Priority),
		PriorityIds:          priorityIds,
		Type:                 itemType,
		CategoryIds:          categoryIds,
		AssigneeIds:          parsePositiveIDList(assigneeIdsText),
		VersionId:            cast.ToInt(query.VersionId),
		VersionIds:           versionIds,
		CreatedBy:            cast.ToInt(query.CreatedBy),
		CreatedByIds:         createdByIds,
		ParentId:             parentId,
		Search:               query.Search,
		TitleSearch:          query.TitleSearch,
		SearchFilters:        searchFilters,
		Timezone:             timezone,
		ExcludeDescendantsOf: cast.ToInt(query.ExcludeDescendantsOf),
		PlanningId:           cast.ToInt(query.PlanningId),
		SortField:            query.SortField,
		SortDirection:        query.SortDirection,
		ViewId:               cast.ToInt(query.ViewId),
		ContextType:          contextType,
		Page:                 cast.ToInt(query.Page),
		Size:                 cast.ToInt(query.Size),
	})
	if err != nil {
		if errors.Is(err, business.ErrViewNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, `view_not_found`))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.list.list_requirements`, err)
		return
	}
	response, err := toListRequirementsResponse(result, lang)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.list.response`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    response,
	})
}

func parseAndValidateSearchFilters(raw string, adminId, wsId, userId int, typeKey string) (string, error) {
	if strings.TrimSpace(raw) == `` {
		return ``, nil
	}
	var tree define.ViewFilterNode
	if err := json.Unmarshal([]byte(raw), &tree); err != nil {
		return ``, errors.New(`search_filters must be valid JSON`)
	}
	if err := validate.ValidateSearchFilters(tree, typeKey); err != nil {
		return ``, err
	}
	if err := business.ValidateWorkItemFilterFields(adminId, cast.ToString(wsId), typeKey, tree); err != nil {
		return ``, err
	}
	encoded, err := json.Marshal(tree)
	if err != nil {
		return ``, err
	}
	return string(encoded), nil
}

func normalizeRequestTimezone(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == `` {
		return `UTC`, nil
	}
	if _, err := time.LoadLocation(value); err != nil {
		return ``, errors.New(`timezone is invalid`)
	}
	return value, nil
}

// parsePositiveIDList parses a comma-separated ID string into a positive-int slice; invalid fragments are already rejected by validatePositiveIDList.
func parsePositiveIDList(value string) []int {
	if strings.TrimSpace(value) == `` {
		return nil
	}
	parts := strings.Split(value, `,`)
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		if id := cast.ToInt(strings.TrimSpace(part)); id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func validatePositiveIDList(value string) error {
	if strings.TrimSpace(value) == `` {
		return nil
	}
	for _, part := range strings.Split(value, `,`) {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			return errors.New(`assignee_ids must contain positive integers`)
		}
	}
	return nil
}

// parseIntegerIDList validates and deduplicates a bounded comma-separated integer list.
func parseIntegerIDList(value string, allowZero bool) ([]int, error) {
	if strings.TrimSpace(value) == `` {
		return nil, nil
	}
	parts := strings.Split(value, `,`)
	if len(parts) > 50 {
		return nil, errors.New(`id list must contain at most 50 values`)
	}
	ids := make([]int, 0, len(parts))
	seen := make(map[int]bool, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id < 0 || (!allowZero && id == 0) {
			return nil, errors.New(`id list contains an invalid integer`)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ListMyWork returns the work items assigned to the current user across workspaces.
func ListMyWork(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))
	result, err := business.ListMyWork(adminId, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `my_work.list`, err)
		return
	}
	response := myWorkResponse{List: toCrossWorkspaceWorkItemResponses(result.Items, lang), CreatableWorkspaces: make([]myWorkWorkspaceResponse, 0, len(result.CreatableWorkspaces)), RecentVisitCount: result.RecentVisitCount}
	for _, workspace := range result.CreatableWorkspaces {
		response.CreatableWorkspaces = append(response.CreatableWorkspaces, myWorkWorkspaceResponse{Id: strconv.Itoa(workspace.Id), Name: workspace.Name, Color: workspace.Color})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}

// ListRecentVisits returns the work items the current user opened within the last week,
// most recently visited first.
func ListRecentVisits(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))
	items, err := business.ListRecentVisits(adminId, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `my_work.recent_visits`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: recentVisitsResponse{List: toCrossWorkspaceWorkItemResponses(items, lang)}})
}

// RecordRequirementVisit stamps the current user's latest visit time on a work item.
func RecordRequirementVisit(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.visit.workspace_id`, err)
		return
	}
	reqId, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.visit.requirement_id`, err)
		return
	}
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))
	if err = business.RecordRequirementVisit(adminId, userId, cast.ToString(wsId), cast.ToString(reqId)); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.visit.record`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// createRequirementRequest is the request body to create a requirement/defect.
type createRequirementRequest struct {
	Title              string                 `json:"title" binding:"required"`
	Description        string                 `json:"description"`
	DescriptionMode    string                 `json:"description_mode"`
	Type               string                 `json:"type"`
	StatusId           any                    `json:"status_id"`
	TemplateId         int                    `json:"template_id" binding:"gte=0"`
	Priority           json.RawMessage        `json:"priority"`
	ParentId           any                    `json:"parent_id"`
	CategoryId         int                    `json:"category_id"`
	AssigneeIds        requirementAssigneeIDs `json:"assignee_ids"`
	VersionId          any                    `json:"version_id"`
	PlannedStartDate   int64                  `json:"planned_start_date"`
	PlannedEndDate     int64                  `json:"planned_end_date"`
	CustomFields       customFieldValues      `json:"custom_fields"`
	PlanningId         int                    `json:"planning_id"`
	ImageFileIds       []int                  `json:"image_file_ids"` // 富文本中先行上传的图片/文件/视频，创建后回绑
	Timezone           string                 `json:"timezone"`       // 触发自动化规则时按该时区计算
	EstimatedWorkHours float64                `json:"estimated_work_hours"`
}

// createRequirementResponse is the response after creating a requirement/defect.
type createRequirementResponse struct {
	Id int `json:"id"`
}

// CreateRequirement creates a requirement.
func CreateRequirement(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.create.workspace_id`, err)
		return
	}

	var req createRequirementRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.create.bind`, err)
		return
	}

	priorityInt := 0
	var priority *int
	if len(req.Priority) > 0 {
		var priorityValue *int
		if err = json.Unmarshal(req.Priority, &priorityValue); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.create.priority`, err)
			return
		}
		if priorityValue != nil {
			priorityInt = *priorityValue
		}
		priority = &priorityInt
	}
	statusInt := cast.ToInt(req.StatusId)
	assigneeIds := []int(req.AssigneeIds)
	if assigneeIds == nil {
		assigneeIds = []int{}
	}
	versionInt := cast.ToInt(req.VersionId)
	parentInt := cast.ToInt(req.ParentId)
	descriptionMode := req.DescriptionMode
	if descriptionMode == `` {
		descriptionMode = define.DescriptionModeRichText
	}
	// Validate via validate layer.
	if err := validate.ValidateRequirementCreate(req.Title, req.Type, priorityInt); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `requirement.create.validate`, err)
		return
	}
	if err := validate.ValidateRequirementDescriptionMode(descriptionMode); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.create.description_mode`, err)
		return
	}
	if err := validate.ValidateTimezone(req.Timezone); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.create.timezone`, err)
		return
	}
	if err := validate.ValidateRequirementWorkHours(req.EstimatedWorkHours); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid), `requirement.create.estimated_work_hours`, err)
		return
	}
	itemType := req.Type
	if itemType == `` {
		itemType = define.TypeKeyRequirement
	}
	if req.CategoryId < 0 || (itemType != define.TypeKeyRequirement && req.CategoryId != 0) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.create.category_id`, errors.New(`category_id must be non-negative and is only valid for requirements`))
		return
	}
	if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), itemType, `create`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}

	model, err := business.CreateRequirement(business.CreateRequirementModel{
		AdminId:            adminId,
		UserId:             userId,
		WorkspaceId:        wsId,
		Title:              req.Title,
		Description:        req.Description,
		DescriptionMode:    descriptionMode,
		Type:               req.Type,
		Priority:           priority,
		StatusId:           statusInt,
		TemplateId:         req.TemplateId,
		ParentId:           parentInt,
		CategoryId:         req.CategoryId,
		AssigneeIds:        assigneeIds,
		VersionId:          versionInt,
		PlannedStartDate:   req.PlannedStartDate,
		PlannedEndDate:     req.PlannedEndDate,
		EstimatedWorkHours: req.EstimatedWorkHours,
		CustomFields:       business.CustomFieldValues(req.CustomFields),
		PlanningId:         req.PlanningId,
		Timezone:           req.Timezone,
		ImageFileIds:       req.ImageFileIds,
	})
	if err != nil {
		if errors.Is(err, validate.ErrPlannedDateRangeInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPlannedDateRangeInvalid), `requirement.create.planned_dates`, err)
			return
		}
		if errors.Is(err, business.ErrWorkItemTemplateNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkItemTemplateNotFound), `requirement.create.template`, err)
			return
		}
		if errors.Is(err, validate.ErrRequirementWorkHoursInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid), `requirement.create.estimated_work_hours`, err)
			return
		}
		if errors.Is(err, business.ErrWorkItemTemplateFieldInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkItemTemplateFieldInvalid), `requirement.create.template_fields`, err)
			return
		}
		if errors.Is(err, business.ErrRequirementRelationInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementRelationInvalid), `requirement.create.relation`, err)
			return
		}
		if errors.Is(err, business.ErrRequirementCategoryNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementCategoryNotFound), `requirement.create.category`, err)
			return
		}
		if errors.Is(err, business.ErrPriorityNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPriorityNotFound), `requirement.create.priority`, err)
			return
		}
		if errors.Is(err, business.ErrStatusNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgStatusNotFound), `requirement.create.status`, err)
			return
		}
		if errors.Is(err, business.ErrVersionNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgVersionNotFound), `requirement.create.version`, err)
			return
		}
		if errors.Is(err, business.ErrVersionEnded) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgVersionEnded), `requirement.create.version_ended`, err)
			return
		}
		if errors.Is(err, business.ErrCustomFieldValueInvalid) {
			abortBadRequest(c, business.CustomFieldValidationMessage(lang, err), `requirement.create.custom_fields`, err)
			return
		}
		if errors.Is(err, business.ErrRequirementAssigneesInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.create.assignees`, err)
			return
		}
		if errors.Is(err, business.ErrPlanningNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPlanningNotFound), `requirement.create.planning`, err)
			return
		}
		if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), `requirement.create.storage`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.create.create_requirement`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    createRequirementResponse{Id: model.Id},
	})
}

// GetRequirement returns requirement details.
func GetRequirement(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.get.workspace_id`, err)
		return
	}
	reqId, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.get.requirement_id`, err)
		return
	}
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))

	// 详情属于读接口，叠加项目角色数据查看范围校验；范围外与不存在统一返回 404。
	result, err := business.GetVisibleRequirementById(adminId, cast.ToString(wsId), cast.ToString(reqId), userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.get.get_requirement_by_id`, err)
		return
	}
	if len(result) == 0 {
		c.JSON(http.StatusNotFound, define.Response{
			Code:    http.StatusNotFound,
			Message: lib_i18n.Show(lang, define.MsgRequirementNotFound),
		})
		return
	}
	if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), result[`type`], `view`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	historyCount, err := business.CountHistoriesByRequirement(adminId, wsId, reqId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.get.count_histories`, err)
		return
	}
	availableStatusIDs, err := business.ListAvailableStatusIDs(adminId, result[`workspace_id`], result[`type`], cast.ToInt(result[`status_id`]))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.get.available_statuses`, err)
		return
	}
	data := requirementResponseItem(result, lang)
	data[`available_status_ids`] = availableStatusIDs
	data[`history_count`] = historyCount
	if result[`type`] == define.TypeKeyRequirement {
		workLogCount, countErr := business.CountRequirementWorkLogs(adminId, wsId, reqId)
		if countErr != nil {
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.get.count_work_logs`, countErr)
			return
		}
		data[`work_log_count`] = workLogCount
	} else {
		data[`work_log_count`] = 0
	}
	customFields, err := business.ListCustomFieldValues(adminId, reqId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.get.custom_fields`, err)
		return
	}
	data[`custom_fields`] = customFields

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    data,
	})
}

// localizeRequirementParams converts the stored workflow keys of a requirement into display text in the requested language.
func localizeRequirementParams(item msql.Params, lang string) {
	item[`status_name`] = lib_i18n.ShowStored(lang, item[`status_name`])
	item[`priority_name`] = lib_i18n.ShowStored(lang, item[`priority_name`])
}

func requirementResponseItem(item msql.Params, lang string) map[string]any {
	localizeRequirementParams(item, lang)
	response := make(map[string]any, len(item))
	for key, value := range item {
		response[key] = value
	}
	for _, field := range []string{`assignee_ids`, `assignee_names`, `assignee_avatars`, `assignee_usernames`} {
		var values any
		if err := json.Unmarshal([]byte(item[field]), &values); err == nil {
			response[field] = values
		} else {
			response[field] = []any{}
		}
	}
	summary := business.CalculateRequirementWorkHours(cast.ToFloat64(item[`work_hours`]), cast.ToFloat64(item[`completed_work_hours`]))
	response[`estimated_work_hours`] = formatRequirementWorkHours(summary.Estimated)
	response[`completed_work_hours`] = formatRequirementWorkHours(summary.Completed)
	response[`remaining_work_hours`] = formatRequirementWorkHours(summary.Remaining)
	response[`overtime_work_hours`] = formatRequirementWorkHours(summary.Overtime)
	response[`work_progress`] = summary.Progress
	return response
}

// UpdateRequirement Update the requirements, only modifying the fields explicitly provided in the request
func UpdateRequirement(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.update.workspace_id`, err)
		return
	}
	reqId, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.update.requirement_id`, err)
		return
	}
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))

	var req struct {
		Title              *string                 `json:"title"`
		Description        *string                 `json:"description"`
		DescriptionMode    *string                 `json:"description_mode"`
		Type               *string                 `json:"type"`
		Priority           json.RawMessage         `json:"priority"`
		StatusId           *int                    `json:"status_id"`
		ParentId           *int                    `json:"parent_id"`
		AssigneeIds        *requirementAssigneeIDs `json:"assignee_ids"`
		VersionId          *int                    `json:"version_id"`
		PlannedStartDate   *int64                  `json:"planned_start_date"`
		PlannedEndDate     *int64                  `json:"planned_end_date"`
		EstimatedWorkHours *float64                `json:"estimated_work_hours"`
		CustomFields       map[string]any          `json:"custom_fields"`
		ImageFileIds       []int                   `json:"image_file_ids"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.update.bind`, err)
		return
	}
	if req.Type != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkItemTypeImmutable), `requirement.update.type`, errors.New(`work item type is immutable`))
		return
	}
	priorityProvided := len(req.Priority) > 0
	priorityInt := 0
	if priorityProvided {
		var priorityValue *int
		if err = json.Unmarshal(req.Priority, &priorityValue); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.update.priority`, err)
			return
		}
		if priorityValue != nil {
			priorityInt = *priorityValue
		}
	}
	if err = validate.ValidateRequirementUpdate(ptrStr(req.Title), ``, priorityInt); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `requirement.update.validate`, err)
		return
	}
	if req.DescriptionMode != nil {
		if err = validate.ValidateRequirementDescriptionMode(*req.DescriptionMode); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.update.description_mode`, err)
			return
		}
	}
	if req.EstimatedWorkHours != nil {
		if err = validate.ValidateRequirementWorkHours(*req.EstimatedWorkHours); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid), `requirement.update.estimated_work_hours`, err)
			return
		}
	}
	current, err := business.GetRequirementById(adminId, cast.ToString(wsId), cast.ToString(reqId))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.update.get_requirement_by_id`, err)
		return
	}
	if len(current) == 0 {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return
	}
	if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), current[`type`], `edit`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}

	// Only update the fields explicitly provided in the request, and if priority is null, it will be cleared to 0.
	updates := map[string]any{}
	if req.Title != nil {
		updates[`title`] = *req.Title
	}
	if req.Description != nil {
		updates[`description`] = *req.Description
	}
	if req.DescriptionMode != nil {
		updates[`description_mode`] = *req.DescriptionMode
	}
	if priorityProvided {
		updates[`priority`] = priorityInt
	}
	if req.StatusId != nil {
		updates[`status_id`] = *req.StatusId
	}
	if req.ParentId != nil {
		updates[`parent_id`] = *req.ParentId
	}
	if req.AssigneeIds != nil {
		updates[`assignee_ids`] = []int(*req.AssigneeIds)
	}
	if req.VersionId != nil {
		updates[`version_id`] = *req.VersionId
	}
	if req.PlannedStartDate != nil {
		updates[`planned_start_date`] = *req.PlannedStartDate
	}
	if req.PlannedEndDate != nil {
		updates[`planned_end_date`] = *req.PlannedEndDate
	}
	if req.EstimatedWorkHours != nil {
		// 缺陷没有预计工时，由 UpdateRequirement 统一归零，这里不拦。
		updates[`work_hours`] = *req.EstimatedWorkHours
	}
	if err = business.UpdateRequirement(adminId, cast.ToString(wsId), cast.ToString(reqId), updates, req.CustomFields, userId, req.ImageFileIds); err != nil {
		if errors.Is(err, business.ErrRequirementNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
			return
		}
		if errors.Is(err, validate.ErrPlannedDateRangeInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPlannedDateRangeInvalid), `requirement.update.planned_dates`, err)
			return
		}
		if errors.Is(err, validate.ErrRequirementWorkHoursInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid), `requirement.update.estimated_work_hours`, err)
			return
		}
		if errors.Is(err, business.ErrRequirementRelationInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementRelationInvalid), `requirement.update.relation`, err)
			return
		}
		if errors.Is(err, business.ErrPriorityNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPriorityNotFound), `requirement.update.priority`, err)
			return
		}
		if errors.Is(err, business.ErrStatusNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgStatusNotFound), `requirement.update.status`, err)
			return
		}
		if errors.Is(err, business.ErrVersionNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgVersionNotFound), `requirement.update.version`, err)
			return
		}
		if errors.Is(err, business.ErrVersionEnded) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgVersionEnded), `requirement.update.version_ended`, err)
			return
		}
		if errors.Is(err, business.ErrCustomFieldValueInvalid) {
			abortBadRequest(c, business.CustomFieldValidationMessage(lang, err), `requirement.update.custom_fields`, err)
			return
		}
		if errors.Is(err, business.ErrRequirementAssigneesInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.update.assignees`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.update.update_requirement`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

const maxRequirementBatchSize = 500

type batchDeleteRequirementsRequest struct {
	Ids []int `json:"ids" binding:"required"`
}

type batchDeleteRequirementsResponse struct {
	DeletedCount int `json:"deleted_count"`
}

func normalizeRequirementBatchIds(input []int) ([]int, error) {
	ids := make([]int, 0, len(input))
	seen := make(map[int]bool, len(input))
	for _, id := range input {
		if id <= 0 {
			return nil, errors.New(`ids must contain positive integers`)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// BatchUpdateRequirements updates a single field of work items in batch.
func BatchUpdateRequirements(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch.workspace_id`, err)
		return
	}
	var req struct {
		Ids   []int           `json:"ids" binding:"required"`
		Field string          `json:"field" binding:"required"`
		Value json.RawMessage `json:"value" binding:"required"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch.bind`, err)
		return
	}
	ids, err := normalizeRequirementBatchIds(req.Ids)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch.ids`, err)
		return
	}
	if len(ids) == 0 || len(ids) > maxRequirementBatchSize || len(req.Value) == 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch.size`, errors.New(`batch size must be between 1 and 500`))
		return
	}
	var value any
	if req.Field == `assignee_ids` {
		var assigneeIds requirementAssigneeIDs
		if err = json.Unmarshal(req.Value, &assigneeIds); err == nil {
			value = []int(assigneeIds)
		}
	} else {
		var valueInt int
		if err = json.Unmarshal(req.Value, &valueInt); err == nil {
			value = valueInt
		}
	}
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch.value`, err)
		return
	}
	if err = validate.ValidateRequirementBatchUpdate(req.Field, value); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch.field`, err)
		return
	}

	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))
	items, err := business.ListRequirementTypesByIDs(adminId, cast.ToString(wsId), ids)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.batch.list_types`, err)
		return
	}
	if len(items) != len(ids) {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return
	}
	for _, item := range items {
		if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), item.Type, `edit`) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
	}

	if err = business.BatchUpdateRequirements(adminId, cast.ToString(wsId), ids, req.Field, value, userId); err != nil {
		switch {
		case errors.Is(err, business.ErrRequirementNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		case errors.Is(err, business.ErrStatusNotFound):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgStatusNotFound), `requirement.batch.status`, err)
		case errors.Is(err, business.ErrPriorityNotFound):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPriorityNotFound), `requirement.batch.priority`, err)
		case errors.Is(err, validate.ErrRequirementBatchInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch.field`, err)
		case errors.Is(err, business.ErrRequirementAssigneesInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch.assignees`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.batch.update`, err)
		}
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    map[string]any{`updated_count`: len(ids)},
	})
}

// BatchDeleteRequirements deletes selected work items in one transaction.
func BatchDeleteRequirements(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch_delete.workspace_id`, err)
		return
	}
	var req batchDeleteRequirementsRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch_delete.bind`, err)
		return
	}
	ids, err := normalizeRequirementBatchIds(req.Ids)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch_delete.ids`, err)
		return
	}
	if len(ids) == 0 || len(ids) > maxRequirementBatchSize {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.batch_delete.size`, errors.New(`batch size must be between 1 and 500`))
		return
	}

	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))
	items, err := business.ListRequirementTypesByIDs(adminId, cast.ToString(wsId), ids)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.batch_delete.list_types`, err)
		return
	}
	if len(items) != len(ids) {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return
	}
	for _, item := range items {
		if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), item.Type, `delete`) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
	}

	if err = business.DeleteRequirements(business.DeleteRequirementsModel{
		AdminId:     adminId,
		WorkspaceId: wsId,
		UserId:      userId,
		Ids:         ids,
	}); err != nil {
		if errors.Is(err, business.ErrRequirementNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.batch_delete.delete_requirements`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    batchDeleteRequirementsResponse{DeletedCount: len(ids)},
	})
}

// DeleteRequirement deletes a requirement.
func DeleteRequirement(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.delete.workspace_id`, err)
		return
	}
	reqId, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.delete.requirement_id`, err)
		return
	}
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))

	current, err := business.GetRequirementById(adminId, cast.ToString(wsId), cast.ToString(reqId))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.delete.get_requirement_by_id`, err)
		return
	}
	if len(current) == 0 {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return
	}
	if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), current[`type`], `delete`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}

	if err = business.DeleteRequirement(adminId, cast.ToString(wsId), cast.ToString(reqId), userId); err != nil {
		if errors.Is(err, business.ErrRequirementNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.delete.delete_requirement`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ListRequirementHistories returns the requirement change-history list.
func ListRequirementHistories(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	reqId := c.Param(`reqId`)
	wsId := cast.ToInt(c.Param(`wsId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))

	// 历史属于读接口，叠加项目角色数据查看范围校验；范围外与不存在统一返回 404。
	current, err := business.GetVisibleRequirementById(adminId, cast.ToString(wsId), reqId, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.history.get_requirement_by_id`, err)
		return
	}
	if len(current) == 0 {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return
	}
	if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), current[`type`], `view`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}

	list, err := business.ListHistoriesByRequirement(business.ListRequirementHistoriesModel{AdminId: adminId, WorkspaceId: wsId, RequirementId: cast.ToInt(reqId)})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.history.list_histories_by_requirement`, err)
		return
	}

	response := make([]requirementHistoryListResponse, 0, len(list))
	for _, item := range list {
		response = append(response, toRequirementHistoryListResponse(item, lang))
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    response,
	})
}

type processingSnapshotResponse struct {
	StatusId      int      `json:"status_id"`
	StatusName    string   `json:"status_name"`
	StatusColor   string   `json:"status_color"`
	AssigneeIds   []int    `json:"assignee_ids"`
	AssigneeNames []string `json:"assignee_names"`
}

type requirementHistoryListResponse struct {
	ProcessingNote        string                      `json:"processing_note"`
	Id                    string                      `json:"id"`
	AdminId               string                      `json:"admin_id"`
	WorkspaceId           string                      `json:"workspace_id"`
	RequirementId         string                      `json:"requirement_id"`
	UserId                string                      `json:"user_id"`
	Action                string                      `json:"action"`
	FieldName             string                      `json:"field_name"`
	OldValue              string                      `json:"old_value"`
	NewValue              string                      `json:"new_value"`
	CreatedAt             string                      `json:"created_at"`
	UserName              string                      `json:"user_name"`
	Source                string                      `json:"source"`
	AutomationRuleId      string                      `json:"automation_rule_id"`
	AutomationExecutionId string                      `json:"automation_execution_id"`
	ActorNameSnapshot     string                      `json:"actor_name_snapshot"`
	ProcessingSnapshot    *processingSnapshotResponse `json:"processing_snapshot"`
	AssigneeNames         []string                    `json:"assignee_names"`
}

func toRequirementHistoryListResponse(item business.RequirementHistoryListItemModel, lang string) requirementHistoryListResponse {
	result := requirementHistoryListResponse{
		ProcessingNote: item.ProcessingNote,
		Id:             strconv.Itoa(item.Id), AdminId: strconv.Itoa(item.AdminId), WorkspaceId: strconv.Itoa(item.WorkspaceId), RequirementId: strconv.Itoa(item.RequirementId), UserId: strconv.Itoa(item.UserId),
		Action: item.Action, FieldName: item.FieldName, OldValue: item.OldValue, NewValue: item.NewValue, CreatedAt: strconv.FormatInt(item.CreatedAt, 10), UserName: item.UserName,
		Source: item.Source, AutomationRuleId: strconv.Itoa(item.AutomationRuleId), AutomationExecutionId: strconv.FormatInt(item.AutomationExecutionId, 10), ActorNameSnapshot: item.ActorNameSnapshot, AssigneeNames: item.AssigneeNames,
	}
	if snapshot := item.ProcessingSnapshot; snapshot != nil {
		result.ProcessingSnapshot = &processingSnapshotResponse{StatusId: snapshot.StatusId, StatusName: lib_i18n.ShowStored(lang, snapshot.StatusName), StatusColor: snapshot.StatusColor, AssigneeIds: snapshot.AssigneeIds, AssigneeNames: snapshot.AssigneeNames}
	}
	return result
}

// GetRequirementHistory returns one change record of a work item.
func GetRequirementHistory(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, reqId, historyId, ok := historyParams(c, lang)
	if !ok {
		return
	}
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))

	// The record belongs to a work item, so the work item has to be visible first; a work item outside
	// the data scope and a missing one both return 404.
	current, err := business.GetVisibleRequirementById(adminId, cast.ToString(wsId), cast.ToString(reqId), userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.history.get_requirement_by_id`, err)
		return
	}
	if len(current) == 0 {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return
	}
	if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), current[`type`], `view`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}

	history, found, err := business.GetHistoryById(adminId, wsId, reqId, historyId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.history.get_history_by_id`, err)
		return
	}
	if !found {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementHistoryNotFound))
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toRequirementHistoryDetailResponse(history, current[`type`], current[`description_mode`]),
	})
}

// requirementHistoryDetailResponse is the change record returned to the change-detail page.
// The work item type travels with it so the page can pick the edit permission for the restore
// button without a second request. description_mode is the work item's current editor mode: the
// mode itself is not versioned, so both sides are rendered with the mode in use now.
type requirementHistoryDetailResponse struct {
	Id              int    `json:"id"`
	Action          string `json:"action"`
	FieldName       string `json:"field_name"`
	OldValue        string `json:"old_value"`
	NewValue        string `json:"new_value"`
	CreatedAt       int64  `json:"created_at"`
	UserName        string `json:"user_name"`
	WorkItemType    string `json:"work_item_type"`
	DescriptionMode string `json:"description_mode"`
}

func toRequirementHistoryDetailResponse(model business.RequirementHistoryModel, workItemType, descriptionMode string) requirementHistoryDetailResponse {
	return requirementHistoryDetailResponse{
		Id:              model.Id,
		Action:          model.Action,
		FieldName:       model.FieldName,
		OldValue:        model.OldValue,
		NewValue:        model.NewValue,
		CreatedAt:       model.CreatedAt,
		UserName:        model.UserName,
		WorkItemType:    workItemType,
		DescriptionMode: descriptionMode,
	}
}

// restoreRequirementHistoryRequest selects which side of the recorded change to restore.
type restoreRequirementHistoryRequest struct {
	Side string `json:"side"`
}

// RestoreRequirementHistory restores the description to the version kept in one change record.
// Restoring is a write, so it reuses the work item edit permission of the item type.
func RestoreRequirementHistory(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, reqId, historyId, ok := historyParams(c, lang)
	if !ok {
		return
	}
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))

	var req restoreRequirementHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.history.restore.bind`, err)
		return
	}
	if err := validate.ValidateRequirementHistoryRestoreSide(req.Side); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.history.restore.side`, err)
		return
	}

	current, err := business.GetVisibleRequirementById(adminId, cast.ToString(wsId), cast.ToString(reqId), userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.history.restore.get_requirement_by_id`, err)
		return
	}
	if len(current) == 0 {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return
	}
	if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), current[`type`], `edit`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}

	err = business.RestoreRequirementDescription(adminId, cast.ToString(wsId), cast.ToString(reqId), historyId, req.Side, userId)
	if err != nil {
		if errors.Is(err, business.ErrRequirementHistoryNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementHistoryNotFound))
			return
		}
		if errors.Is(err, business.ErrRequirementHistoryNotRestorable) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementHistoryInvalid), `requirement.history.restore.unsupported`, err)
			return
		}
		if errors.Is(err, business.ErrRequirementNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.history.restore`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgRequirementHistoryRestored),
	})
}

// historyParams parses the three path parameters shared by the change-history endpoints.
func historyParams(c *gin.Context, lang string) (int, int, int, bool) {
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.history.workspace_id`, err)
		return 0, 0, 0, false
	}
	reqId, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.history.requirement_id`, err)
		return 0, 0, 0, false
	}
	historyId, err := positiveParam(c, `historyId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement.history.history_id`, err)
		return 0, 0, 0, false
	}
	return wsId, reqId, historyId, true
}

func hasWorkItemPermission(adminId int, userId int, wsId string, itemType string, action string) bool {
	return business.HasPermission(adminId, userId, wsId, workItemPermissionKey(itemType, action))
}

func workItemPermissionKey(itemType string, action string) string {
	switch itemType {
	case define.TypeKeyDefect:
		return `defect:` + action
	case define.TypeKeyIteration:
		return `version:` + action
	default:
		return `requirement:` + action
	}
}
