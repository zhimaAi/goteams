// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

type versionWorkspaceResponse struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	IsOrigin bool   `json:"is_origin"`
}

// versionResponse is an iteration returned to the client.
type versionResponse struct {
	Id               int                        `json:"id"`
	AdminId          int                        `json:"admin_id"`
	WorkspaceId      int                        `json:"workspace_id"`
	Name             string                     `json:"name"`
	StartDate        int64                      `json:"start_date"`
	EndDate          int64                      `json:"end_date"`
	EndedAt          int64                      `json:"ended_at"`
	IsEnded          bool                       `json:"is_ended"`
	IterationType    string                     `json:"iteration_type"`
	Workspaces       []versionWorkspaceResponse `json:"workspaces"`
	RequirementCount int                        `json:"requirement_count"`
	CompletedCount   int                        `json:"completed_count"`
	CreatedAt        int64                      `json:"created_at"`
	UpdatedAt        int64                      `json:"updated_at"`
}

type createVersionRequest struct {
	Name          string `json:"name" binding:"required"`
	StartDate     int64  `json:"start_date"`
	EndDate       int64  `json:"end_date"`
	IterationType string `json:"iteration_type"`
	WorkspaceIds  []int  `json:"workspace_ids"`
}

type createVersionResponse struct {
	Id int `json:"id"`
}

type updateVersionRequest struct {
	Name          string `json:"name" binding:"required"`
	StartDate     int64  `json:"start_date"`
	EndDate       int64  `json:"end_date"`
	IterationType string `json:"iteration_type" binding:"required"`
	WorkspaceIds  []int  `json:"workspace_ids"`
}

type updateVersionEndedRequest struct {
	Ended *bool `json:"ended" binding:"required"`
}

type versionWorkspaceOptionResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type iterationFilterStatusResponse struct {
	Id            int    `json:"id"`
	WorkspaceId   int    `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	Type          string `json:"type"`
	Name          string `json:"name"`
	Color         string `json:"color"`
}

type iterationFilterPriorityResponse struct {
	Id            int    `json:"id"`
	WorkspaceId   int    `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	Type          string `json:"type"`
	Name          string `json:"name"`
	Color         string `json:"color"`
}

type iterationFilterMemberResponse struct {
	UserId   int    `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

type iterationFilterOptionsResponse struct {
	Workspaces     []versionWorkspaceResponse        `json:"workspaces"`
	Statuses       []iterationFilterStatusResponse   `json:"statuses"`
	Priorities     []iterationFilterPriorityResponse `json:"priorities"`
	Members        []iterationFilterMemberResponse   `json:"members"`
	FinalStatusIds []int                             `json:"final_status_ids"`
}

type listVersionWorkItemsQuery struct {
	ViewId        string `form:"view_id"`
	SearchFilters string `form:"search_filters"`
	Timezone      string `form:"timezone"`
	Page          string `form:"page"`
	Size          string `form:"size"`
	SortField     string `form:"sort_field"`
	SortDirection string `form:"sort_direction"`
}

type versionWorkItemResponseItem struct {
	Id                string   `json:"id"`
	WorkspaceId       string   `json:"workspace_id"`
	WorkspaceName     string   `json:"workspace_name"`
	ParentId          string   `json:"parent_id"`
	Title             string   `json:"title"`
	Type              string   `json:"type"`
	Priority          string   `json:"priority"`
	StatusId          string   `json:"status_id"`
	CreatedBy         string   `json:"created_by"`
	VersionId         string   `json:"version_id"`
	PlannedStartDate  string   `json:"planned_start_date"`
	PlannedEndDate    string   `json:"planned_end_date"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
	StatusName        string   `json:"status_name"`
	StatusColor       string   `json:"status_color"`
	PriorityName      string   `json:"priority_name"`
	PriorityColor     string   `json:"priority_color"`
	CreatorName       string   `json:"creator_name"`
	AssigneeIds       []int    `json:"assignee_ids"`
	AssigneeNames     []string `json:"assignee_names"`
	AssigneeAvatars   []string `json:"assignee_avatars"`
	AssigneeUsernames []string `json:"assignee_usernames"`
}

type listVersionWorkItemsResponse struct {
	List  []versionWorkItemResponseItem `json:"list"`
	Total int                           `json:"total"`
	Page  int                           `json:"page"`
	Size  int                           `json:"size"`
}

// toVersionsResponse converts version models to the response list.
func toVersionsResponse(models []business.VersionModel) []versionResponse {
	items := make([]versionResponse, 0, len(models))
	for _, model := range models {
		workspaces := make([]versionWorkspaceResponse, 0, len(model.Workspaces))
		for _, workspace := range model.Workspaces {
			workspaces = append(workspaces, versionWorkspaceResponse{
				Id: workspace.Id, Name: workspace.Name, IsOrigin: workspace.IsOrigin,
			})
		}
		items = append(items, versionResponse{
			Id: model.Id, AdminId: model.AdminId, WorkspaceId: model.WorkspaceId,
			Name: model.Name, StartDate: model.StartDate, EndDate: model.EndDate,
			EndedAt: model.EndedAt, IsEnded: model.EndedAt > 0, IterationType: model.IterationType(),
			Workspaces: workspaces, RequirementCount: model.RequirementCount, CompletedCount: model.CompletedCount,
			CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
		})
	}
	return items
}

// ListVersions returns the iteration list of a workspace.
func ListVersions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.list.workspace_id`, err)
		return
	}
	list, err := business.ListVersionsByWorkspace(c.GetInt(`adminId`), workspaceID)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `version.list.list_versions_by_workspace`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toVersionsResponse(list)})
}

// ListVersionWorkspaceOptions returns workspaces where the requester can create iterations.
func ListVersionWorkspaceOptions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	if _, err := positiveParam(c, `wsId`); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.workspace_options.workspace_id`, err)
		return
	}
	items, err := business.ListVersionWorkspaceOptions(c.GetInt(`adminId`), c.GetInt(`userId`))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `version.workspace_options.list`, err)
		return
	}
	response := make([]versionWorkspaceOptionResponse, 0, len(items))
	for _, item := range items {
		response = append(response, versionWorkspaceOptionResponse{Id: item.Id, Name: item.Name})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}

// CreateVersion creates an iteration.
func CreateVersion(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.create.workspace_id`, err)
		return
	}
	var req createVersionRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.create.bind`, err)
		return
	}
	if req.IterationType == `` {
		req.IterationType = define.IterationTypeSingle
	}
	if err = validateVersionPayload(req.Name, req.StartDate, req.EndDate, req.IterationType); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `version.create.validate`, err)
		return
	}
	result, err := business.CreateVersion(business.CreateVersionModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), WorkspaceId: workspaceID,
		Name: req.Name, StartDate: req.StartDate, EndDate: req.EndDate,
		IterationType: req.IterationType, WorkspaceIds: req.WorkspaceIds,
	})
	if err != nil {
		respondVersionError(c, lang, `version.create.create_version`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: createVersionResponse{Id: result.Id},
	})
}

// UpdateVersion updates iteration details and associated workspaces.
func UpdateVersion(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, versionID, err := versionRouteIDs(c)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.update.id`, err)
		return
	}
	var req updateVersionRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.update.bind`, err)
		return
	}
	if err = validateVersionPayload(req.Name, req.StartDate, req.EndDate, req.IterationType); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `version.update.validate`, err)
		return
	}
	err = business.UpdateVersion(business.UpdateVersionModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), WorkspaceId: workspaceID,
		VersionId: versionID, Name: req.Name, StartDate: req.StartDate, EndDate: req.EndDate,
		IterationType: req.IterationType, WorkspaceIds: req.WorkspaceIds,
	})
	if err != nil {
		respondVersionError(c, lang, `version.update.update_version`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// UpdateVersionEnded ends or reopens an iteration.
func UpdateVersionEnded(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, versionID, err := versionRouteIDs(c)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.ended.id`, err)
		return
	}
	var req updateVersionEndedRequest
	if err = c.ShouldBindJSON(&req); err != nil || req.Ended == nil {
		if err == nil {
			err = errors.New(`ended is required`)
		}
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.ended.bind`, err)
		return
	}
	if err = business.UpdateVersionEnded(business.VersionActionModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: workspaceID, VersionId: versionID,
	}, *req.Ended); err != nil {
		respondVersionError(c, lang, `version.ended.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// DeleteVersion deletes an iteration and removes its work-item associations.
func DeleteVersion(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, versionID, err := versionRouteIDs(c)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.delete.id`, err)
		return
	}
	if err = business.DeleteVersion(business.VersionActionModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: workspaceID, VersionId: versionID,
	}); err != nil {
		respondVersionError(c, lang, `version.delete.delete_version`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// GetIterationFilterOptions returns visible workspace/status/priority/member filter sources.
func GetIterationFilterOptions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, versionID, err := versionRouteIDs(c)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.filter_options.id`, err)
		return
	}
	model, err := business.GetIterationFilterOptions(c.GetInt(`adminId`), c.GetInt(`userId`), workspaceID, versionID)
	if err != nil {
		respondVersionError(c, lang, `version.filter_options.get`, err)
		return
	}
	workspaceNames := make(map[int]string, len(model.Workspaces))
	workspaces := make([]versionWorkspaceResponse, 0, len(model.Workspaces))
	for _, workspace := range model.Workspaces {
		workspaceNames[workspace.Id] = workspace.Name
		workspaces = append(workspaces, versionWorkspaceResponse{Id: workspace.Id, Name: workspace.Name, IsOrigin: workspace.IsOrigin})
	}
	statuses := make([]iterationFilterStatusResponse, 0, len(model.Statuses))
	for _, item := range model.Statuses {
		statuses = append(statuses, iterationFilterStatusResponse{
			Id: item.Id, WorkspaceId: item.WorkspaceId, WorkspaceName: workspaceNames[item.WorkspaceId],
			Type: item.Type, Name: lib_i18n.ShowStored(lang, item.Name), Color: item.Color,
		})
	}
	priorities := make([]iterationFilterPriorityResponse, 0, len(model.Priorities))
	for _, item := range model.Priorities {
		priorities = append(priorities, iterationFilterPriorityResponse{
			Id: item.Id, WorkspaceId: item.WorkspaceId, WorkspaceName: workspaceNames[item.WorkspaceId],
			Type: item.Type, Name: lib_i18n.ShowStored(lang, item.Name), Color: item.Color,
		})
	}
	members := make([]iterationFilterMemberResponse, 0, len(model.Members))
	for _, item := range model.Members {
		members = append(members, iterationFilterMemberResponse{
			UserId: item.UserId, Name: item.Name, Username: item.Username, Avatar: item.Avatar,
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: iterationFilterOptionsResponse{
			Workspaces: workspaces, Statuses: statuses, Priorities: priorities,
			Members: members, FinalStatusIds: model.FinalStatusIds,
		},
	})
}

// ListVersionWorkItems returns visible work items from every associated workspace.
func ListVersionWorkItems(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, versionID, err := versionRouteIDs(c)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.work_items.id`, err)
		return
	}
	var query listVersionWorkItemsQuery
	if err = c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.work_items.bind`, err)
		return
	}
	// The iteration list sorts through the same SQL builder as the requirement list, so
	// the shared whitelist keeps every sortable column reachable without widening it here.
	if query.SortField != `` && !business.IsRequirementSortField(query.SortField) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.work_items.sort_field`, errors.New(`invalid sort field`))
		return
	}
	if query.SortDirection != `` && query.SortDirection != `asc` && query.SortDirection != `desc` {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.work_items.sort_direction`, errors.New(`invalid sort direction`))
		return
	}
	searchFilters, err := parseAndValidateSearchFilters(
		query.SearchFilters, c.GetInt(`adminId`), workspaceID, c.GetInt(`userId`), define.TypeKeyIteration,
	)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.work_items.search_filters`, err)
		return
	}
	timezone, err := normalizeRequestTimezone(query.Timezone)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `version.work_items.timezone`, err)
		return
	}
	model, err := business.ListRequirements(business.ListRequirementsModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: workspaceID, UserId: c.GetInt(`userId`),
		VersionId: versionID, ViewId: parsePositiveInt(query.ViewId), ContextType: define.TypeKeyIteration,
		SearchFilters: searchFilters, Timezone: timezone, SortField: query.SortField,
		SortDirection: query.SortDirection, Page: parsePositiveInt(query.Page), Size: parsePositiveInt(query.Size),
	})
	if err != nil {
		switch {
		case errors.Is(err, business.ErrVersionNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgVersionNotFound))
		case errors.Is(err, business.ErrViewNotFound):
			abortNotFound(c, lib_i18n.Show(lang, `view_not_found`))
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `version.work_items.list`, err)
		}
		return
	}
	items := make([]versionWorkItemResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, versionWorkItemResponseItem{
			Id: strconv.Itoa(item.Id), WorkspaceId: strconv.Itoa(item.WorkspaceId), WorkspaceName: item.WorkspaceName,
			ParentId: strconv.Itoa(item.ParentId), Title: item.Title, Type: item.Type,
			Priority: strconv.Itoa(item.Priority), StatusId: strconv.Itoa(item.StatusId), CreatedBy: strconv.Itoa(item.CreatedBy),
			VersionId: strconv.Itoa(item.VersionId), PlannedStartDate: strconv.FormatInt(item.PlannedStartDate, 10),
			PlannedEndDate: strconv.FormatInt(item.PlannedEndDate, 10), CreatedAt: strconv.FormatInt(item.CreatedAt, 10),
			UpdatedAt: strconv.FormatInt(item.UpdatedAt, 10), StatusName: lib_i18n.ShowStored(lang, item.StatusName),
			StatusColor: item.StatusColor, PriorityName: lib_i18n.ShowStored(lang, item.PriorityName), PriorityColor: item.PriorityColor,
			CreatorName: item.CreatorName, AssigneeIds: item.AssigneeIds, AssigneeNames: item.AssigneeNames,
			AssigneeAvatars: item.AssigneeAvatars, AssigneeUsernames: item.AssigneeUsernames,
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: listVersionWorkItemsResponse{List: items, Total: model.Total, Page: model.Page, Size: model.Size},
	})
}

func parsePositiveInt(value string) int {
	value = strings.TrimSpace(value)
	if value == `` {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func validateVersionPayload(name string, startDate, endDate int64, iterationType string) error {
	if err := validate.ValidateVersionCreate(name); err != nil {
		return err
	}
	if err := validate.ValidateVersionDates(startDate, endDate); err != nil {
		return &validate.FieldError{Field: "dates", Key: "common_param_invalid"}
	}
	return validate.ValidateIterationType(iterationType)
}

func versionRouteIDs(c *gin.Context) (int, int, error) {
	workspaceID, err := positiveParam(c, `wsId`)
	if err != nil {
		return 0, 0, err
	}
	versionID, err := positiveParam(c, `versionId`)
	return workspaceID, versionID, err
}

func respondVersionError(c *gin.Context, lang, stage string, err error) {
	switch {
	case errors.Is(err, business.ErrVersionNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgVersionNotFound))
	case errors.Is(err, business.ErrVersionNameExists):
		logRequestError(c, stage, err)
		abortConflict(c, lib_i18n.Show(lang, define.MsgVersionNameExist))
	case errors.Is(err, business.ErrVersionWorkspaceInvalid), errors.Is(err, business.ErrVersionTypeImmutable):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}
