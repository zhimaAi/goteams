// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// ======================== Work Item Views ========================

// workItemViewResponseItem is one view in the list response; numeric fields
// keep the existing contract (output as strings) and filters is raw JSON.
type workItemViewResponseItem struct {
	Id         string          `json:"id"`
	Name       string          `json:"name"`
	ViewType   string          `json:"view_type"`
	BuiltinKey string          `json:"builtin_key"`
	Filters    json.RawMessage `json:"filters"`
	CreatedBy  string          `json:"created_by"`
	Enabled    bool            `json:"enabled"`
	IsDefault  bool            `json:"is_default"`
}

// workItemViewListResponse is the view list plus the user's remember switch.
type workItemViewListResponse struct {
	List           []workItemViewResponseItem `json:"list"`
	RememberLast   bool                       `json:"remember_last"`
	SearchExpanded bool                       `json:"search_expanded"`
	SearchFilters  json.RawMessage            `json:"search_filters"`
}

// saveWorkItemViewRequest is the create/update payload; Type and ViewType are
// only honored on create (both are immutable afterwards). Gantt views ignore
// Filters and store the three independent sections instead.
type saveWorkItemViewRequest struct {
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	ViewType string                 `json:"view_type"`
	Filters  *define.ViewFilterNode `json:"filters"`
	// Gantt-only filter sections (common/requirement/defect).
	CommonFilters      *define.ViewFilterNode `json:"common_filters"`
	RequirementFilters *define.ViewFilterNode `json:"requirement_filters"`
	DefectFilters      *define.ViewFilterNode `json:"defect_filters"`
}

// saveUserViewSettingsRequest is the per-user view preferences payload.
type saveUserViewSettingsRequest struct {
	Type              string                 `json:"type"`
	DefaultViewId     *int                   `json:"default_view_id"`
	DisabledViewIds   *[]int                 `json:"disabled_view_ids"`
	PersonalViewOrder *[]int                 `json:"personal_view_order"`
	RememberLast      *bool                  `json:"remember_last"`
	SearchExpanded    *bool                  `json:"search_expanded"`
	SearchFilters     *define.ViewFilterNode `json:"search_filters"`
}

// marshalViewFilters serializes the request filter tree for storage (” when absent).
func marshalViewFilters(filters *define.ViewFilterNode) (string, error) {
	if filters == nil {
		return ``, nil
	}
	raw, err := json.Marshal(filters)
	if err != nil {
		return ``, err
	}
	return string(raw), nil
}

// marshalGanttViewFilters validates each gantt section structurally and
// serializes the stored three-section payload (” when all sections are empty).
func marshalGanttViewFilters(req saveWorkItemViewRequest) (define.GanttViewFilters, string, error) {
	sections := define.GanttViewFilters{Common: req.CommonFilters, Requirement: req.RequirementFilters, Defect: req.DefectFilters}
	for _, node := range []*define.ViewFilterNode{sections.Common, sections.Requirement, sections.Defect} {
		if node == nil {
			continue
		}
		if err := validate.ValidateViewFilters(*node); err != nil {
			return sections, ``, err
		}
	}
	if sections.Common != nil {
		if err := validate.ValidateGanttCommonFilterFields(*sections.Common); err != nil {
			return sections, ``, err
		}
	}
	if sections.Common == nil && sections.Requirement == nil && sections.Defect == nil {
		return sections, ``, nil
	}
	raw, err := json.Marshal(sections)
	if err != nil {
		return sections, ``, err
	}
	return sections, string(raw), nil
}

// viewFiltersRaw returns stored filters as raw JSON for the response (null when empty).
func viewFiltersRaw(filters string) json.RawMessage {
	if len(filters) == 0 {
		return nil
	}
	return json.RawMessage(filters)
}

// requestViewType reads the ?type= work item type with the same default as the route middleware.
func requestViewType(c *gin.Context) string {
	return c.DefaultQuery(`type`, define.TypeKeyRequirement)
}

// ListWorkItemViews returns the view list (system + own personal) merged with user preferences.
func ListWorkItemViews(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.list.workspace_id`, err)
		return
	}
	typeKey := requestViewType(c)
	if err = validate.ValidateViewContextType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.list.type`, err)
		return
	}
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)

	result, err := business.ListWorkItemViews(adminId, cast.ToString(wsId), typeKey, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `view.list.query`, err)
		return
	}
	items := make([]workItemViewResponseItem, 0, len(result.Views))
	for _, view := range result.Views {
		items = append(items, workItemViewResponseItem{
			Id:         strconv.Itoa(view.Id),
			Name:       lib_i18n.ShowStored(lang, view.Name),
			ViewType:   view.ViewType,
			BuiltinKey: view.BuiltinKey,
			Filters:    viewFiltersRaw(view.Filters),
			CreatedBy:  strconv.Itoa(view.CreatedBy),
			Enabled:    view.Enabled,
			IsDefault:  view.IsDefault,
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: workItemViewListResponse{
			List:           items,
			RememberLast:   result.RememberLast,
			SearchExpanded: result.SearchExpanded,
			SearchFilters:  json.RawMessage(result.SearchFilters),
		},
	})
}

// bindViewPayload binds and validates the create/update payload (name + filters).
func bindViewPayload(c *gin.Context, lang, stage string) (saveWorkItemViewRequest, string, bool) {
	var req saveWorkItemViewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.bind`, err)
		return req, ``, false
	}
	if err := validate.ValidateViewName(req.Name); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.name`, err)
		return req, ``, false
	}
	if req.Filters != nil {
		if err := validate.ValidateViewFilters(*req.Filters); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.filters`, err)
			return req, ``, false
		}
	}
	filters, err := marshalViewFilters(req.Filters)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.filters_encode`, err)
		return req, ``, false
	}
	return req, filters, true
}

// CreateWorkItemView creates a personal or system view (system requires manage permission).
func CreateWorkItemView(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.create.workspace_id`, err)
		return
	}
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)

	req, filters, ok := bindViewPayload(c, lang, `view.create`)
	if !ok {
		return
	}
	if err = validate.ValidateViewContextType(req.Type); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.create.type`, err)
		return
	}
	var ganttSections define.GanttViewFilters
	if req.Type == define.TypeKeyGantt {
		ganttSections, filters, err = marshalGanttViewFilters(req)
		if err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.create.gantt_filters`, err)
			return
		}
	} else if req.Filters != nil {
		if err = validate.ValidateViewFiltersForType(*req.Filters, req.Type); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.create.filters_type`, err)
			return
		}
	}
	if err = validate.ValidateViewType(req.ViewType); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.create.view_type`, err)
		return
	}
	// Creating any view requires at least view access to the module; system views need manage permission.
	if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), req.Type, `view`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	if req.ViewType == define.ViewTypeSystem && !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), req.Type, `manage_system_view`) {
		abortForbidden(c, lib_i18n.Show(lang, `view_no_system_perm`))
		return
	}
	if req.Type == define.TypeKeyGantt {
		if err = business.ValidateGanttViewFilterSections(adminId, cast.ToString(wsId), ganttSections); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.create.gantt_filters_fields`, err)
			return
		}
	} else if req.Filters != nil {
		if err = business.ValidateWorkItemFilterFields(adminId, cast.ToString(wsId), req.Type, *req.Filters); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.create.filters_fields`, err)
			return
		}
	}

	id, err := business.CreateWorkItemView(adminId, cast.ToString(wsId), req.Type, req.Name, req.ViewType, filters, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `view.create.insert`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, `view_created`),
		Data:    map[string]any{`id`: strconv.Itoa(id)},
	})
}

// loadManagedView loads a view and enforces edit/delete access rules:
// builtin "all" is locked for everyone, system views need the manage permission,
// personal views are only accessible to their creator.
func loadManagedView(c *gin.Context, lang string, adminId, userId, wsId int) (business.WorkItemViewModel, bool) {
	viewId, err := positiveParam(c, `viewId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.manage.view_id`, err)
		return business.WorkItemViewModel{}, false
	}
	view, err := business.GetWorkItemViewById(adminId, cast.ToString(wsId), viewId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `view.manage.get`, err)
		return business.WorkItemViewModel{}, false
	}
	if view.Id == 0 {
		abortNotFound(c, lib_i18n.Show(lang, `view_not_found`))
		return business.WorkItemViewModel{}, false
	}
	if view.BuiltinKey == define.ViewBuiltinAll {
		abortForbidden(c, lib_i18n.Show(lang, `view_builtin_all_locked`))
		return business.WorkItemViewModel{}, false
	}
	if view.ViewType == define.ViewTypeSystem {
		if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), view.TypeKey, `manage_system_view`) {
			abortForbidden(c, lib_i18n.Show(lang, `view_no_system_perm`))
			return business.WorkItemViewModel{}, false
		}
		return view, true
	}
	// Personal views: reply 404 for other users' views to avoid leaking their existence.
	if view.CreatedBy != userId {
		abortNotFound(c, lib_i18n.Show(lang, `view_not_found`))
		return business.WorkItemViewModel{}, false
	}
	return view, true
}

// UpdateWorkItemView updates the name/filters of an editable view.
func UpdateWorkItemView(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.update.workspace_id`, err)
		return
	}
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)

	view, ok := loadManagedView(c, lang, adminId, userId, wsId)
	if !ok {
		return
	}
	req, filters, ok := bindViewPayload(c, lang, `view.update`)
	if !ok {
		return
	}
	if view.TypeKey == define.TypeKeyGantt {
		var ganttSections define.GanttViewFilters
		ganttSections, filters, err = marshalGanttViewFilters(req)
		if err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.update.gantt_filters`, err)
			return
		}
		if err = business.ValidateGanttViewFilterSections(adminId, cast.ToString(wsId), ganttSections); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.update.gantt_filters_fields`, err)
			return
		}
	} else if req.Filters != nil {
		if err = validate.ValidateViewFiltersForType(*req.Filters, view.TypeKey); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.update.filters_type`, err)
			return
		}
		if err = business.ValidateWorkItemFilterFields(adminId, cast.ToString(wsId), view.TypeKey, *req.Filters); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.update.filters_fields`, err)
			return
		}
	}
	if err = business.UpdateWorkItemView(adminId, cast.ToString(wsId), view, req.Name, filters); err != nil {
		if errors.Is(err, business.ErrViewBuiltinLocked) {
			abortForbidden(c, lib_i18n.Show(lang, `view_builtin_all_locked`))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `view.update.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, `view_updated`),
	})
}

// DeleteWorkItemView deletes an editable view.
func DeleteWorkItemView(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.delete.workspace_id`, err)
		return
	}
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)

	view, ok := loadManagedView(c, lang, adminId, userId, wsId)
	if !ok {
		return
	}
	if err = business.DeleteWorkItemView(adminId, cast.ToString(wsId), view); err != nil {
		if errors.Is(err, business.ErrViewBuiltinLocked) {
			abortForbidden(c, lib_i18n.Show(lang, `view_builtin_all_locked`))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `view.delete.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, `view_deleted`),
	})
}

// SaveUserViewSettings upserts the requesting user's view preferences for one module.
func SaveUserViewSettings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.settings.workspace_id`, err)
		return
	}
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)

	var req saveUserViewSettingsRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.settings.bind`, err)
		return
	}
	if err = validate.ValidateViewContextType(req.Type); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.settings.type`, err)
		return
	}
	if !hasWorkItemPermission(adminId, userId, cast.ToString(wsId), req.Type, `view`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	var searchFilters *string
	if req.SearchFilters != nil {
		if err = validate.ValidateSearchFilters(*req.SearchFilters, req.Type); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.settings.search_filters`, err)
			return
		}
		if err = business.ValidateWorkItemFilterFields(adminId, cast.ToString(wsId), req.Type, *req.SearchFilters); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.settings.search_filter_fields`, err)
			return
		}
		encoded, encodeErr := marshalViewFilters(req.SearchFilters)
		if encodeErr != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `view.settings.search_filters_encode`, encodeErr)
			return
		}
		searchFilters = &encoded
	}
	err = business.SaveUserViewSettings(business.UserViewSettingsPatch{
		AdminId:           adminId,
		WorkspaceId:       cast.ToString(wsId),
		TypeKey:           req.Type,
		UserId:            userId,
		DefaultViewId:     req.DefaultViewId,
		DisabledViewIds:   req.DisabledViewIds,
		PersonalViewOrder: req.PersonalViewOrder,
		RememberLast:      req.RememberLast,
		SearchExpanded:    req.SearchExpanded,
		SearchFilters:     searchFilters,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `view.settings.save`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, `view_settings_saved`),
	})
}
