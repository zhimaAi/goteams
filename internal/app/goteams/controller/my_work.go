package controller

import (
	"strconv"

	"goteams/internal/app/goteams/business"
	"goteams/internal/pkg/lib_i18n"
)

type crossWorkspaceWorkItemResponse struct {
	Id                string   `json:"id"`
	AdminId           string   `json:"admin_id"`
	WorkspaceId       string   `json:"workspace_id"`
	ParentId          string   `json:"parent_id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Type              string   `json:"type"`
	Priority          string   `json:"priority"`
	StatusId          string   `json:"status_id"`
	VersionId         string   `json:"version_id"`
	PlannedStartDate  string   `json:"planned_start_date"`
	PlannedEndDate    string   `json:"planned_end_date"`
	WorkHours         string   `json:"work_hours"`
	CommentCount      string   `json:"comment_count"`
	CreatedBy         string   `json:"created_by"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
	WorkspaceName     string   `json:"workspace_name"`
	WorkspaceColor    string   `json:"workspace_color"`
	StatusName        string   `json:"status_name"`
	StatusColor       string   `json:"status_color"`
	StatusSortOrder   string   `json:"status_sort_order"`
	PriorityName      string   `json:"priority_name"`
	PriorityColor     string   `json:"priority_color"`
	PrioritySortOrder string   `json:"priority_sort_order"`
	AssigneeIds       []int    `json:"assignee_ids"`
	AssigneeNames     []string `json:"assignee_names"`
	AssigneeAvatars   []string `json:"assignee_avatars"`
	AssigneeUsernames []string `json:"assignee_usernames"`
	ParentTitle       string   `json:"parent_title"`
	IsDone            string   `json:"is_done"`
	IsCompleted       bool     `json:"is_completed"`
	CanEdit           string   `json:"can_edit"`
	VisitedAt         string   `json:"visited_at,omitempty"`
	requirementWorkHoursSummaryResponse
}
type myWorkWorkspaceResponse struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}
type myWorkResponse struct {
	List                []crossWorkspaceWorkItemResponse `json:"list"`
	CreatableWorkspaces []myWorkWorkspaceResponse        `json:"creatable_workspaces"`
	RecentVisitCount    int                              `json:"recent_visit_count"`
}
type recentVisitsResponse struct {
	List []crossWorkspaceWorkItemResponse `json:"list"`
}

func toCrossWorkspaceWorkItemResponses(items []business.CrossWorkspaceWorkItemModel, lang string) []crossWorkspaceWorkItemResponse {
	result := make([]crossWorkspaceWorkItemResponse, 0, len(items))
	for _, item := range items {
		response := crossWorkspaceWorkItemResponse{}
		response.Id = strconv.Itoa(item.Id)
		response.AdminId = strconv.Itoa(item.AdminId)
		response.WorkspaceId = strconv.Itoa(item.WorkspaceId)
		if item.ParentId != nil {
			response.ParentId = strconv.Itoa(*item.ParentId)
		}
		response.Title = item.Title
		response.Description = item.Description
		response.Type = item.Type
		if item.Priority != nil {
			response.Priority = strconv.Itoa(*item.Priority)
		}
		if item.StatusId != nil {
			response.StatusId = strconv.Itoa(*item.StatusId)
		}
		if item.VersionId != nil {
			response.VersionId = strconv.Itoa(*item.VersionId)
		}
		response.PlannedStartDate = strconv.FormatInt(item.PlannedStartDate, 10)
		response.PlannedEndDate = strconv.FormatInt(item.PlannedEndDate, 10)
		estimatedHours := 0.0
		if item.WorkHours != nil {
			estimatedHours = *item.WorkHours
			response.WorkHours = strconv.FormatFloat(*item.WorkHours, 'f', 2, 64)
		}
		if item.CommentCount != nil {
			response.CommentCount = strconv.Itoa(*item.CommentCount)
		}
		if item.CreatedBy != nil {
			response.CreatedBy = strconv.Itoa(*item.CreatedBy)
		}
		response.CreatedAt = strconv.FormatInt(item.CreatedAt, 10)
		response.UpdatedAt = strconv.FormatInt(item.UpdatedAt, 10)
		response.WorkspaceName = item.WorkspaceName
		response.WorkspaceColor = item.WorkspaceColor
		response.StatusName = lib_i18n.ShowStored(lang, item.StatusName)
		response.StatusColor = item.StatusColor
		if item.StatusSortOrder != nil {
			response.StatusSortOrder = strconv.Itoa(*item.StatusSortOrder)
		}
		response.PriorityName = lib_i18n.ShowStored(lang, item.PriorityName)
		response.PriorityColor = item.PriorityColor
		if item.PrioritySortOrder != nil {
			response.PrioritySortOrder = strconv.Itoa(*item.PrioritySortOrder)
		}
		response.AssigneeIds = item.AssigneeIds
		response.AssigneeNames = item.AssigneeNames
		response.AssigneeAvatars = item.AssigneeAvatars
		response.AssigneeUsernames = item.AssigneeUsernames
		response.ParentTitle = item.ParentTitle
		response.IsDone = strconv.FormatBool(item.IsDone)
		response.IsCompleted = item.IsCompleted
		response.CanEdit = "0"
		if item.CanEdit {
			response.CanEdit = "1"
		}
		if item.VisitedAt != nil {
			response.VisitedAt = strconv.FormatInt(*item.VisitedAt, 10)
		}
		response.requirementWorkHoursSummaryResponse = toRequirementWorkHoursSummaryResponse(business.CalculateRequirementWorkHours(estimatedHours, item.CompletedWorkHours))
		result = append(result, response)
	}
	return result
}
