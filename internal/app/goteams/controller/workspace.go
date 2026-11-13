// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"net/url"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/tool"
)

// ======================== Workspace CRUD ========================

// listWorkspacesResponseItem keeps the historical JSON contract of the project list (all values are strings).
type listWorkspacesResponseItem struct {
	Id              string `json:"id"`
	WorkspaceType   string `json:"workspace_type"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	Description     string `json:"description"`
	Color           string `json:"color"`
	Icon            string `json:"icon"`
	ActiveVersionId string `json:"active_version_id"`
	CreatedBy       string `json:"created_by"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// toListWorkspacesResponse converts business models into project list response items.
func toListWorkspacesResponse(model business.ListWorkspacesModel) []listWorkspacesResponseItem {
	items := make([]listWorkspacesResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, listWorkspacesResponseItem{
			WorkspaceType:   item.WorkspaceType,
			Id:              cast.ToString(item.Id),
			Name:            item.Name,
			Slug:            item.Slug,
			Description:     item.Description,
			Color:           item.Color,
			Icon:            item.Icon,
			ActiveVersionId: cast.ToString(item.ActiveVersionId),
			CreatedBy:       cast.ToString(item.CreatedBy),
			CreatedAt:       cast.ToString(item.CreatedAt),
			UpdatedAt:       cast.ToString(item.UpdatedAt),
		})
	}
	return items
}

// ListWorkspaces returns all workspaces the user participates in.
func ListWorkspaces(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model, err := business.ListWorkspaces(business.ListWorkspacesModel{
		AdminId: c.GetInt(`adminId`),
		UserId:  c.GetInt(`userId`),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.list.list_workspaces`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListWorkspacesResponse(model),
	})
}

type createWorkspaceRequest struct {
	Name                string `json:"name" binding:"required"`
	Description         string `json:"description"`
	Slug                string `json:"slug"`
	Color               string `json:"color"`
	TemplateWorkspaceId int    `json:"template_workspace_id"`
	WorkspaceType       string `json:"workspace_type"`
}

type createWorkspaceResponse struct {
	Id int `json:"id"`
}

// CreateWorkspace creates a workspace.
func CreateWorkspace(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req createWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.create.bind`, err)
		return
	}

	// Validate via validate layer.
	if err := validate.ValidateWorkspaceCreate(req.Name, req.Description, req.Color); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `workspace.create.validate`, err)
		return
	}

	if req.WorkspaceType == "" {
		req.WorkspaceType = define.WorkspaceTypeSoftware
	}
	if err := validate.ValidateWorkspaceType(req.WorkspaceType); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.create.type`, err)
		return
	}
	wsId, err := business.CreateWorkspace(business.CreateWorkspaceModel{
		AdminId: adminId, Name: req.Name, Description: req.Description,
		Slug: req.Slug, Color: req.Color, CreatedBy: userId, WorkspaceType: req.WorkspaceType,
		TemplateWorkspaceId: req.TemplateWorkspaceId,
	})
	if err != nil {
		if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), `workspace.create.quota`, err) {
			return
		}
		if errors.Is(err, business.ErrWorkspaceTypeInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkspaceTypeInvalid), `workspace.create.type_invalid`, err)
			return
		}
		if errors.Is(err, business.ErrWorkspaceSlugExists) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkspaceSlugExist), `workspace.create.slug_exists`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.create.create_workspace`, err)
		return
	}

	// If a template workspace was specified, copy its configuration. A failed copy
	// degrades to the already-created workspace's seed configuration.
	if req.TemplateWorkspaceId > 0 {
		if err := business.CopyTemplateFromWorkspace(define.CopyWorkspaceTemplateModel{
			AdminID:             adminId,
			OperatorID:          userId,
			TemplateWorkspaceID: req.TemplateWorkspaceId,
			NewWorkspaceID:      wsId,
		}); err != nil {
			logs.Error("copy template [%d] to workspace [%d] failed: %s", req.TemplateWorkspaceId, wsId, err.Error())
		}
	}

	// Keep the creator's newly created project at the top of their personal project list.
	if err := business.SortWorkspaces(business.SortWorkspacesModel{
		AdminId: adminId,
		UserId:  userId,
		WsIds:   []int{wsId},
	}); err != nil {
		logs.Error("prepend created workspace [%d] to user order failed: %s", wsId, err.Error())
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    createWorkspaceResponse{Id: wsId},
	})
}

// GetWorkspace returns workspace details.
func GetWorkspace(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId := cast.ToInt(c.GetInt(`adminId`))

	result, err := business.GetWorkspaceById(adminId, wsId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.get.get_workspace_by_id`, err)
		return
	}
	if len(result) == 0 {
		c.JSON(http.StatusNotFound, define.Response{
			Code:    http.StatusNotFound,
			Message: lib_i18n.Show(lang, define.MsgWorkspaceNotFound),
		})
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    result,
	})
}

// UpdateWorkspace updates a workspace.
func UpdateWorkspace(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.update.bind`, err)
		return
	}

	// Validate via validate layer.
	if err := validate.ValidateWorkspaceUpdate(req.Name, req.Description, req.Color); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `workspace.update.validate`, err)
		return
	}

	if err := business.UpdateWorkspace(adminId, wsId, req.Name, req.Description, req.Color); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.update.update_workspace`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// GetWorkspaceIcon returns the workspace icon file.
func GetWorkspaceIcon(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	path, err := business.GetWorkspaceIconPath(c.GetInt(`adminId`), c.Param(`wsId`))
	if err != nil {
		if errors.Is(err, business.ErrWorkspaceIconNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceIconNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.icon.get_path`, err)
		return
	}
	c.Header(`Cache-Control`, `no-store`)
	if err := lib_storage.M.Serve(path, c); err != nil {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceIconNotFound))
	}
}

// UploadWorkspaceIcon uploads and updates the workspace icon.
func UploadWorkspaceIcon(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	file, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkspaceIconInvalid), `workspace.icon.form_file`, err)
		return
	}
	icon, err := business.SaveWorkspaceIcon(c.GetInt(`adminId`), c.Param(`wsId`), file)
	if err != nil {
		switch {
		case errors.Is(err, business.ErrWorkspaceIconInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkspaceIconInvalid), `workspace.icon.invalid`, err)
		case errors.Is(err, business.ErrWorkspaceIconTooLarge):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkspaceIconTooLarge), `workspace.icon.too_large`, err)
		case errors.Is(err, business.ErrWorkspaceIconNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceNotFound))
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.icon.save`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    gin.H{`icon`: icon},
	})
}

// DeleteWorkspaceIcon removes the workspace icon.
func DeleteWorkspaceIcon(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	if err := business.DeleteWorkspaceIcon(c.GetInt(`adminId`), c.Param(`wsId`)); err != nil {
		if errors.Is(err, business.ErrWorkspaceIconNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.icon.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeleteWorkspace permanently deletes a project together with all of its data.
func DeleteWorkspace(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model := business.DeleteWorkspaceModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    cast.ToInt(c.Param(`wsId`)),
	}
	if err := business.DeleteWorkspace(model); err != nil {
		if errors.Is(err, business.ErrWorkspaceNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.delete.delete_workspace`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ListWorkspaceTemplates returns template workspaces from the configured template admin account.
func ListWorkspaceTemplates(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	templates, err := business.ListWorkspaceTemplates()
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.templates.list`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListWorkspacesResponse(business.ListWorkspacesModel{Items: templates}),
	})
}

// GetWorkspaceTemplateIcon serves one template project's avatar out of the offline package. The
// route carries no authentication: the create-project dialog loads the card images as plain <img>
// requests, which cannot carry the Authorization header, and the avatar is already part of the
// public package download. A miss is a 404, which the dialog covers with the letter avatar.
func GetWorkspaceTemplateIcon(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	data, asset, ok := business.TemplateWorkspaceIcon(cast.ToInt(c.Param(`wsId`)))
	if !ok {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceIconNotFound))
		return
	}
	contentType := asset.ContentType
	if contentType == `` {
		contentType = `application/octet-stream`
	}
	c.Header(`Cache-Control`, `no-store`)
	c.Data(http.StatusOK, contentType, data)
}

// ExportWorkspaceTemplates packages every copyable configuration of the template account as a
// downloadable tar.gz. The caller carries a signed short-lived `?token=` instead of a session:
// both consumers (a browser address bar and a headless curl) can attach a query string but not the
// Authorization header. Anyone holding a valid token may download, and a deployment without a
// template account answers 403 as well.
func ExportWorkspaceTemplates(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	templateAdminId := business.GetTemplateAdminId()
	// Forbidden means forbidden: the status alone, no message body. The package carries the whole
	// template account, so the caller is told nothing beyond "denied".
	if templateAdminId == 0 || !business.VerifyTemplateExportToken(c.Query(`token`), tool.Time2Int()) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	result, err := business.ExportTemplatePackage(business.ExportTemplatePackageModel{AdminID: templateAdminId})
	if err != nil {
		switch {
		case errors.Is(err, business.ErrTemplateExportForbidden):
			c.AbortWithStatus(http.StatusForbidden)
		case errors.Is(err, business.ErrTemplatePackageTooLarge):
			abortInternal(c, lib_i18n.Show(lang, define.MsgTemplatePackageTooLarge), `workspace.templates.export.too_large`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.templates.export`, err)
		}
		return
	}

	// Headers before any body byte; the fixed name lets an operator drop the file straight onto
	// the package path, and no-store keeps a proxy from serving an older package.
	c.Header(`Content-Disposition`, `attachment; filename*=UTF-8''`+url.PathEscape(result.Filename))
	c.Header(`Cache-Control`, `no-store`)
	c.Header(`X-Content-Type-Options`, `nosniff`)
	c.Data(http.StatusOK, `application/gzip`, result.Data)
}

// ======================== Enterprise Project Management ========================

type listEnterpriseProjectsQuery struct {
	Keyword string `form:"keyword"`
}

type listEnterpriseProjectsResponseItem struct {
	Id          int      `json:"id"`
	Name        string   `json:"name"`
	Color       string   `json:"color"`
	Icon        string   `json:"icon"`
	EndedAt     int64    `json:"ended_at"`
	SortOrder   int      `json:"sort_order"`
	CreatorName string   `json:"creator_name"`
	AdminNames  []string `json:"admin_names"`
	MemberCount int      `json:"member_count"`
	CreatedAt   int64    `json:"created_at"`
}

type updateWorkspaceEndedRequest struct {
	Ended *bool `json:"ended" binding:"required"`
}

type sortEnterpriseProjectsRequest struct {
	Ids        []int `json:"ids" binding:"required"`
	ApplyToAll bool  `json:"apply_to_all"`
}

// toListEnterpriseProjectsResponse converts business models into project management response items.
func toListEnterpriseProjectsResponse(model business.ListEnterpriseProjectsModel) []listEnterpriseProjectsResponseItem {
	items := make([]listEnterpriseProjectsResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, listEnterpriseProjectsResponseItem{
			Id:          item.Id,
			Name:        item.Name,
			Color:       item.Color,
			Icon:        item.Icon,
			EndedAt:     item.EndedAt,
			SortOrder:   item.SortOrder,
			CreatorName: item.CreatorName,
			AdminNames:  item.AdminNames,
			MemberCount: item.MemberCount,
			CreatedAt:   item.CreatedAt,
		})
	}
	return items
}

// ListEnterpriseProjects returns all tenant projects for the project management page.
func ListEnterpriseProjects(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var query listEnterpriseProjectsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.enterprise_projects.bind`, err)
		return
	}
	if err := validate.MaxLen(`keyword`, query.Keyword, 200); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.enterprise_projects.validate`, err)
		return
	}

	model, err := business.ListEnterpriseProjects(business.ListEnterpriseProjectsModel{
		AdminId: c.GetInt(`adminId`),
		UserId:  c.GetInt(`userId`),
		Keyword: query.Keyword,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.enterprise_projects.list`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListEnterpriseProjectsResponse(model),
	})
}

// UpdateWorkspaceEnded ends or reopens a project.
func UpdateWorkspaceEnded(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req updateWorkspaceEndedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.ended.bind`, err)
		return
	}

	err := business.SetWorkspaceEnded(business.SetWorkspaceEndedModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    cast.ToInt(c.Param(`wsId`)),
		Ended:   *req.Ended,
	})
	if err != nil {
		if errors.Is(err, business.ErrWorkspaceNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.ended.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// SortEnterpriseProjects saves the project display order used by both the management page and the sidebar.
func SortEnterpriseProjects(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req sortEnterpriseProjectsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.sort.bind`, err)
		return
	}

	err := business.SortWorkspaces(business.SortWorkspacesModel{
		AdminId:    c.GetInt(`adminId`),
		UserId:     c.GetInt(`userId`),
		WsIds:      req.Ids,
		ApplyToAll: req.ApplyToAll,
	})
	if err != nil {
		if errors.Is(err, business.ErrWorkspaceOrderForbidden) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
		if errors.Is(err, business.ErrWorkspaceOrderInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.sort.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.sort.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}
