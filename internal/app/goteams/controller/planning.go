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
	"github.com/spf13/cast"
)

// planningGroupNameMaxLen is the max length of a group name, kept in sync with the frontend maxlength.
const planningGroupNameMaxLen = 20

// ======================== Planning group DTO ========================

type listPlanningGroupsResponseItem struct {
	Id        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type savePlanningGroupRequest struct {
	Name string `json:"name"`
}

type createPlanningGroupResponse struct {
	Id int `json:"id"`
}

// ======================== Planning row DTO ========================

type planningRelationResponse struct {
	Id    int    `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type listPlanningsResponseItem struct {
	Id               int                        `json:"id"`
	GroupId          int                        `json:"group_id"`
	Title            string                     `json:"title"`
	Color            string                     `json:"color"`
	TitleStrike      int                        `json:"title_strike"`
	IsArchived       int                        `json:"is_archived"`
	Priority         int                        `json:"priority"`
	StatusId         int                        `json:"status_id"`
	AssigneeIds      []int                      `json:"assignee_ids"`
	PlannedStartDate int64                      `json:"planned_start_date"`
	PlannedEndDate   int64                      `json:"planned_end_date"`
	SortOrder        int64                      `json:"sort_order"`
	CreatedBy        int                        `json:"created_by"`
	CreatedAt        int64                      `json:"created_at"`
	UpdatedAt        int64                      `json:"updated_at"`
	Relations        []planningRelationResponse `json:"relations"`
	CustomFields     map[string]any             `json:"custom_fields"`
}

type listPlanningsResponse struct {
	List    []listPlanningsResponseItem `json:"list"`
	HasMore bool                        `json:"has_more"`
}

type listPlanningsQuery struct {
	GroupId         int    `form:"group_id"`
	Title           string `form:"title"`
	Priority        string `form:"priority"`
	StatusId        string `form:"status_id"`
	AssigneeId      int    `form:"assignee_id"`
	HasRelation     string `form:"has_relation"`
	Archived        string `form:"archived"`
	CursorId        int    `form:"cursor_id"`
	CursorSortOrder int64  `form:"cursor_sort_order"`
	Size            int    `form:"size"`
}

// createPlanningRequest creates a planning row; title may be empty, position set by prev_id/next_id.
type createPlanningRequest struct {
	GroupId          int            `json:"group_id"`
	Title            *string        `json:"title"`
	Color            *string        `json:"color"`
	TitleStrike      *int           `json:"title_strike"`
	IsArchived       *int           `json:"is_archived"`
	Priority         *int           `json:"priority"`
	StatusId         *int           `json:"status_id"`
	AssigneeIds      *[]int         `json:"assignee_ids"`
	PlannedStartDate *int64         `json:"planned_start_date"`
	PlannedEndDate   *int64         `json:"planned_end_date"`
	CustomFields     map[string]any `json:"custom_fields"`
	PrevId           int            `json:"prev_id"`
	NextId           int            `json:"next_id"`
}

// updatePlanningRequest edits inline; nil fields mean "not updated this time".
type updatePlanningRequest struct {
	Title            *string        `json:"title"`
	Color            *string        `json:"color"`
	TitleStrike      *int           `json:"title_strike"`
	IsArchived       *int           `json:"is_archived"`
	Priority         *int           `json:"priority"`
	StatusId         *int           `json:"status_id"`
	AssigneeIds      *[]int         `json:"assignee_ids"`
	PlannedStartDate *int64         `json:"planned_start_date"`
	PlannedEndDate   *int64         `json:"planned_end_date"`
	CustomFields     map[string]any `json:"custom_fields"`
}

type movePlanningRequest struct {
	GroupId int `json:"group_id"`
	PrevId  int `json:"prev_id"`
	NextId  int `json:"next_id"`
}

type savePlanningRelationsRequest struct {
	ItemIds []int `json:"item_ids"`
}

// toPlanningResponseItem converts a single planning model into a response item.
func toPlanningResponseItem(item business.PlanningModel) listPlanningsResponseItem {
	relations := make([]planningRelationResponse, 0, len(item.Relations))
	for _, relation := range item.Relations {
		relations = append(relations, planningRelationResponse{Id: relation.Id, Title: relation.Title, Type: relation.Type})
	}
	customFields := map[string]any{}
	for key, value := range item.CustomFields {
		customFields[key] = value
	}
	return listPlanningsResponseItem{
		Id: item.Id, GroupId: item.GroupId, Title: item.Title, Color: item.Color,
		TitleStrike: item.TitleStrike, IsArchived: item.IsArchived,
		Priority: item.Priority, StatusId: item.StatusId, AssigneeIds: item.AssigneeIds,
		PlannedStartDate: item.PlannedStartDate, PlannedEndDate: item.PlannedEndDate,
		SortOrder: item.SortOrder, CreatedBy: item.CreatedBy, CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt, Relations: relations, CustomFields: customFields,
	}
}

// toListPlanningsResponse converts planning models into a response.
func toListPlanningsResponse(model business.ListPlanningsModel) listPlanningsResponse {
	list := make([]listPlanningsResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		list = append(list, toPlanningResponseItem(item))
	}
	return listPlanningsResponse{List: list, HasMore: model.HasMore}
}

// validatePlanningGroupName validates the group name is non-empty and within length.
func validatePlanningGroupName(name string) error {
	if fe := validate.NotBlank("name", name); fe != nil {
		return fe
	}
	if fe := validate.MaxLen("name", name, planningGroupNameMaxLen); fe != nil {
		return fe
	}
	return nil
}

// abortPlanningError maps a planning domain error to the corresponding HTTP response.
func abortPlanningError(c *gin.Context, lang, scene string, err error) bool {
	if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), scene, err) {
		return true
	}
	switch {
	case errors.Is(err, business.ErrPlanningNotFound):
		c.JSON(http.StatusNotFound, define.Response{Code: http.StatusNotFound, Message: lib_i18n.Show(lang, define.MsgPlanningNotFound)})
	case errors.Is(err, business.ErrPlanningGroupNotFound):
		c.JSON(http.StatusNotFound, define.Response{Code: http.StatusNotFound, Message: lib_i18n.Show(lang, define.MsgPlanningGroupNotFound)})
	case errors.Is(err, business.ErrPlanningGroupLastOne):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPlanningGroupLastOne), scene, err)
	case errors.Is(err, business.ErrPlanningRelationInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPlanningRelationInvalid), scene, err)
	case errors.Is(err, business.ErrStatusNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgStatusNotFound), scene, err)
	case errors.Is(err, business.ErrPriorityNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPriorityNotFound), scene, err)
	case errors.Is(err, business.ErrCustomFieldValueInvalid):
		abortBadRequest(c, business.CustomFieldValidationMessage(lang, err), scene, err)
	case errors.Is(err, validate.ErrPlannedDateRangeInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPlannedDateRangeInvalid), scene, err)
	default:
		return false
	}
	return true
}

// ======================== Planning group APIs ========================

// ListPlanningGroups returns the planning groups under the workspace.
func ListPlanningGroups(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model, err := business.ListPlanningGroups(business.ListPlanningGroupsModel{
		AdminId:     c.GetInt(`adminId`),
		WorkspaceId: c.Param(`wsId`),
		Lang:        lang,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning_group.list`, err)
		return
	}
	items := make([]listPlanningGroupsResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, listPlanningGroupsResponseItem{
			Id: item.Id, Name: item.Name, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    items,
	})
}

// CreatePlanningGroup adds a planning group.
func CreatePlanningGroup(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req savePlanningGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning_group.create.bind`, err)
		return
	}
	if err := validatePlanningGroupName(req.Name); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning_group.create.validate`, err)
		return
	}
	model, err := business.CreatePlanningGroup(business.CreatePlanningGroupModel{
		AdminId:     c.GetInt(`adminId`),
		WorkspaceId: c.Param(`wsId`),
		Name:        req.Name,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning_group.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    createPlanningGroupResponse{Id: model.Id},
	})
}

// UpdatePlanningGroup renames a planning group.
func UpdatePlanningGroup(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req savePlanningGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning_group.update.bind`, err)
		return
	}
	if err := validatePlanningGroupName(req.Name); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning_group.update.validate`, err)
		return
	}
	if err := business.UpdatePlanningGroup(business.UpdatePlanningGroupModel{
		AdminId:     c.GetInt(`adminId`),
		WorkspaceId: c.Param(`wsId`),
		GroupId:     cast.ToInt(c.Param(`groupId`)),
		Name:        req.Name,
	}); err != nil {
		if abortPlanningError(c, lang, `planning_group.update`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning_group.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeletePlanningGroup moves a planning group and all its planning data into the recycle bin.
func DeletePlanningGroup(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	if err := business.DeletePlanningGroup(business.DeletePlanningGroupModel{
		AdminId:     c.GetInt(`adminId`),
		UserId:      c.GetInt(`userId`),
		WorkspaceId: c.Param(`wsId`),
		GroupId:     cast.ToInt(c.Param(`groupId`)),
	}); err != nil {
		if abortPlanningError(c, lang, `planning_group.delete`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning_group.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ======================== Planning row APIs ========================

// ListPlannings returns the planning list by group (keyset cursor pagination).
func ListPlannings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var query listPlanningsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.list.bind`, err)
		return
	}
	priorityIds, err := parsePlanningFilterIDs(query.Priority)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.list.priority`, err)
		return
	}
	statusIds, err := parsePlanningFilterIDs(query.StatusId)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.list.status`, err)
		return
	}
	archived, err := parsePlanningArchivedFilter(query.Archived)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.list.archived`, err)
		return
	}

	model := business.ListPlanningsModel{
		AdminId:         c.GetInt(`adminId`),
		WorkspaceId:     c.Param(`wsId`),
		GroupId:         query.GroupId,
		Lang:            lang,
		Title:           query.Title,
		PriorityIds:     priorityIds,
		StatusIds:       statusIds,
		AssigneeId:      query.AssigneeId,
		Archived:        archived,
		CursorId:        query.CursorId,
		CursorSortOrder: query.CursorSortOrder,
		Size:            query.Size,
	}
	if query.HasRelation != `` {
		hasRelation, err := strconv.ParseBool(query.HasRelation)
		if err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.list.has_relation`, err)
			return
		}
		model.HasRelation = &hasRelation
	}

	result, err := business.ListPlannings(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListPlanningsResponse(result),
	})
}

// parsePlanningFilterIDs parses a comma-separated list of positive IDs and removes duplicates.
func parsePlanningFilterIDs(value string) ([]int, error) {
	if strings.TrimSpace(value) == `` {
		return nil, nil
	}
	parts := strings.Split(value, `,`)
	ids := make([]int, 0, len(parts))
	seen := make(map[int]bool, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			return nil, errors.New(`planning filter IDs must contain positive integers`)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// parsePlanningArchivedFilter parses the archive filter: empty or "0" keeps unarchived rows,
// "1" keeps archived rows, "all" keeps both.
func parsePlanningArchivedFilter(value string) (*int, error) {
	switch strings.TrimSpace(value) {
	case ``, `0`:
		unarchived := 0
		return &unarchived, nil
	case `1`:
		archived := 1
		return &archived, nil
	case `all`:
		return nil, nil
	default:
		return nil, errors.New(`planning archived filter must be 0, 1 or all`)
	}
}

// CreatePlanning creates a planning row.
func CreatePlanning(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req createPlanningRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.create.bind`, err)
		return
	}
	if req.GroupId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.create.group`, nil)
		return
	}
	model, err := business.CreatePlanning(business.SavePlanningModel{
		AdminId:          c.GetInt(`adminId`),
		UserId:           c.GetInt(`userId`),
		WorkspaceId:      c.Param(`wsId`),
		GroupId:          req.GroupId,
		Title:            req.Title,
		Color:            req.Color,
		TitleStrike:      req.TitleStrike,
		IsArchived:       req.IsArchived,
		Priority:         req.Priority,
		StatusId:         req.StatusId,
		AssigneeIds:      req.AssigneeIds,
		PlannedStartDate: req.PlannedStartDate,
		PlannedEndDate:   req.PlannedEndDate,
		CustomFields:     business.CustomFieldValues(req.CustomFields),
		PrevId:           req.PrevId,
		NextId:           req.NextId,
	})
	if err != nil {
		if abortPlanningError(c, lang, `planning.create`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toPlanningResponseItem(model.Item),
	})
}

// UpdatePlanning edits planning fields inline.
func UpdatePlanning(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req updatePlanningRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.update.bind`, err)
		return
	}
	if err := business.UpdatePlanning(business.SavePlanningModel{
		AdminId:          c.GetInt(`adminId`),
		UserId:           c.GetInt(`userId`),
		WorkspaceId:      c.Param(`wsId`),
		PlanningId:       cast.ToInt(c.Param(`planningId`)),
		Title:            req.Title,
		Color:            req.Color,
		TitleStrike:      req.TitleStrike,
		IsArchived:       req.IsArchived,
		Priority:         req.Priority,
		StatusId:         req.StatusId,
		AssigneeIds:      req.AssigneeIds,
		PlannedStartDate: req.PlannedStartDate,
		PlannedEndDate:   req.PlannedEndDate,
		CustomFields:     business.CustomFieldValues(req.CustomFields),
	}); err != nil {
		if abortPlanningError(c, lang, `planning.update`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeletePlanning deletes a planning row.
func DeletePlanning(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	if err := business.DeletePlanning(business.DeletePlanningModel{
		AdminId:     c.GetInt(`adminId`),
		UserId:      c.GetInt(`userId`),
		WorkspaceId: c.Param(`wsId`),
		PlanningId:  cast.ToInt(c.Param(`planningId`)),
	}); err != nil {
		if abortPlanningError(c, lang, `planning.delete`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// MovePlanning reorders planning rows via drag-and-drop.
func MovePlanning(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req movePlanningRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.move.bind`, err)
		return
	}
	if req.GroupId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.move.group`, nil)
		return
	}
	if err := business.MovePlanning(business.MovePlanningModel{
		AdminId:     c.GetInt(`adminId`),
		WorkspaceId: c.Param(`wsId`),
		PlanningId:  cast.ToInt(c.Param(`planningId`)),
		GroupId:     req.GroupId,
		PrevId:      req.PrevId,
		NextId:      req.NextId,
	}); err != nil {
		if abortPlanningError(c, lang, `planning.move`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.move`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// SavePlanningRelations overwrites the planning's related work items.
func SavePlanningRelations(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req savePlanningRelationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.relations.bind`, err)
		return
	}
	if err := business.SavePlanningRelations(business.SavePlanningRelationsModel{
		AdminId:     c.GetInt(`adminId`),
		WorkspaceId: c.Param(`wsId`),
		PlanningId:  cast.ToInt(c.Param(`planningId`)),
		ItemIds:     req.ItemIds,
	}); err != nil {
		if abortPlanningError(c, lang, `planning.relations`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.relations`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}
