// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

type uploadEnterpriseAvatarResponse struct {
	Url string `json:"url"`
}

type listEnterpriseMembersResponseItem struct {
	Id            string   `json:"id"`
	Username      string   `json:"username"`
	Name          string   `json:"name"`
	Phone         string   `json:"phone"`
	Email         string   `json:"email"`
	Avatar        string   `json:"avatar"`
	RoleId        string   `json:"role_id"`
	Status        string   `json:"status"`
	IsOwner       bool     `json:"is_owner"`
	InvitedByLink bool     `json:"invited_by_link"`
	DepartmentIds []string `json:"department_ids"`
	CreatedAt     string   `json:"created_at"`
}

type createEnterpriseMemberRequest struct {
	Username      string `json:"username" binding:"required"`
	Password      string `json:"password" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	RoleId        int    `json:"roleId" binding:"required,gt=0"`
	Avatar        string `json:"avatar"`
	DepartmentIds []int  `json:"departmentIds"`
}

type createEnterpriseMemberResponse struct {
	UserId int `json:"userId"`
}

type updateEnterpriseMemberRequest struct {
	Username      string `json:"username" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	RoleId        int    `json:"roleId" binding:"required,gt=0"`
	Avatar        string `json:"avatar"`
	DepartmentIds []int  `json:"departmentIds"`
}

// ======================== Permission API ========================

// PermissionItem is the API response for a single permission definition.
type PermissionItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Group string `json:"group"`
}

// ListPermissions returns all permission groups for a given scope (enterprise/project).
func ListPermissions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope := c.DefaultQuery("scope", "project")

	// Enterprise-level returns team work, pipeline, project, members_enterprise, and members_roles groups.
	// Pipeline permissions are placed above the project group per product requirement.
	// Pipeline permissions carry no wsId and are resolved against the enterprise role, so they can only be configured here.
	// Project-level returns the permission groups configurable by project roles, avoiding mixing in enterprise member permissions.
	var groups []permission.Group
	// Permission names embed the built-in work item type wording, so project scope follows the
	// workspace's work item attribute config (e.g. 查看需求 → 查看工单); enterprise scope carries
	// no requirement/defect permission group and stays on the built-in wording.
	workItemNames := map[string]string{}
	if scope == permission.ScopeEnterprise {
		for _, key := range []string{
			permission.GroupKeyTeamWork,
			permission.GroupKeyPipeline,
			permission.GroupKeyProject,
			permission.GroupKeyPlan,
			permission.GroupKeyMemberEnterprise,
			permission.GroupKeyMemberRoles,
			permission.GroupKeyStorage,
			permission.GroupKeyExternalNotify,
			permission.GroupKeySso,
			permission.GroupKeyPersonalization,
		} {
			g, ok := permission.GetGroup(key)
			if ok {
				groups = append(groups, g)
			}
		}
	} else {
		projectGroupKeys := []string{
			permission.GroupKeyPlanning,
			permission.GroupKeyRequirement,
			permission.GroupKeyDefect,
			permission.GroupKeyGantt,
			permission.GroupKeyVersion,
			permission.GroupKeyKnowledgeBase,
			permission.GroupKeyProjectInfo,
			permission.GroupKeyProjectMembers,
			permission.GroupKeyProjectRoles,
			permission.GroupKeyRequirementSetup,
			permission.GroupKeyDefectSetup,
			permission.GroupKeyAutomation,
			permission.GroupKeyApplicationMarket,
		}
		wsID := cast.ToInt(c.Query(`wsId`))
		adminID := c.GetInt(`adminId`)
		if wsID > 0 {
			issueWorkspace, err := business.IsIssueWorkspace(adminID, wsID)
			if err != nil {
				abortInternal(c, lib_i18n.Show(lang, define.MsgError), `permission.workspace_type`, err)
				return
			}
			if issueWorkspace {
				automationIndex := slices.Index(projectGroupKeys, permission.GroupKeyAutomation)
				projectGroupKeys = slices.Insert(projectGroupKeys, automationIndex+1, permission.GroupKeySLA)
			}
			enabledApplications, err := business.GetEnabledApplicationKeys(adminID, wsID)
			if err != nil {
				abortInternal(c, lib_i18n.Show(lang, define.MsgError), `permission.applications`, err)
				return
			}
			projectGroupKeys = append(projectGroupKeys, permission.GroupKeyTicket, permission.GroupKeyDashboard)
			projectGroupKeys = slices.DeleteFunc(projectGroupKeys, func(key string) bool {
				return !business.IsWorkspacePermissionGroupEnabled(key, enabledApplications)
			})
			attributes, err := business.ListWorkItemAttributes(business.ListWorkItemAttributesModel{
				AdminId: adminID,
				WsId:    cast.ToString(wsID),
			})
			if err != nil {
				abortInternal(c, lib_i18n.Show(lang, define.MsgError), `permission.work_item_attribute`, err)
				return
			}
			for _, attribute := range attributes.Items {
				if len(attribute.Name) > 0 {
					workItemNames[attribute.TypeKey] = attribute.Name
				}
			}
		}
		for _, key := range projectGroupKeys {
			g, ok := permission.GetGroup(key)
			if ok {
				groups = append(groups, g)
			}
		}
	}

	type GroupResp struct {
		Group string           `json:"group"`
		Items []PermissionItem `json:"items"`
	}

	result := make([]GroupResp, 0, len(groups))
	for _, g := range groups {
		groupName := permission.ApplyWorkItemNames(permission.GetGroupName(g.Key, lang), lang, workItemNames)
		items := make([]PermissionItem, 0, len(g.Permissions))
		for _, p := range g.Permissions {
			items = append(items, PermissionItem{
				Key:   p.Key,
				Label: permission.ApplyWorkItemNames(permission.GetPermissionName(p.Key, lang), lang, workItemNames),
				Group: groupName,
			})
		}
		result = append(result, GroupResp{
			Group: groupName,
			Items: items,
		})
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    result,
	})
}

// ======================== Enterprise Member Management ========================

// UploadEnterpriseAvatar uploads and temporarily stores the avatar for a pending enterprise member.
func UploadEnterpriseAvatar(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	file, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarInvalid), `enterprise.member.avatar.form_file`, err)
		return
	}
	avatar, err := business.SavePendingAvatar(c.GetInt(`adminId`), file)
	if err != nil {
		switch {
		case errors.Is(err, business.ErrAvatarInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarInvalid), `enterprise.member.avatar.invalid`, err)
		case errors.Is(err, business.ErrAvatarTooLarge):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarTooLarge), `enterprise.member.avatar.too_large`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.member.avatar.save`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    uploadEnterpriseAvatarResponse{Url: avatar},
	})
}

// ListEnterpriseMembers returns all users under the current tenant (admin_id isolated).
func ListEnterpriseMembers(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, err := business.ListEnterpriseMembers(business.ListEnterpriseMembersModel{
		AdminId: c.GetInt(`adminId`),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.member.list.list_enterprise_members`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListEnterpriseMembersResponse(model),
	})
}

// CreateEnterpriseMember creates a new user and assigns an enterprise role.
func CreateEnterpriseMember(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req createEnterpriseMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.member.create.bind`, err)
		return
	}
	if err := validateEnterpriseMemberFields(req.Username, req.Name, req.Phone, req.Email); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `enterprise.member.create.validate`, err)
		return
	}

	model, err := business.CreateEnterpriseMember(business.CreateEnterpriseMemberModel{
		AdminId: c.GetInt(`adminId`), OperatorUserId: c.GetInt(`userId`), Username: req.Username, Password: req.Password, Name: req.Name,
		Phone: req.Phone, Email: req.Email, RoleId: req.RoleId, Avatar: req.Avatar,
		DepartmentIds: req.DepartmentIds,
	})
	if err != nil {
		// 云版配额优先判断；账号重名由下方已有的 ErrUserUsernameExists 分支处理。
		if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), `enterprise.member.create.quota`, err) {
			return
		}
		if errors.Is(err, business.ErrAvatarInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarInvalid), `enterprise.member.create.avatar_claim`, err)
			return
		}
		if errors.Is(err, business.ErrUserUsernameExists) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgUserExist), `enterprise.member.create.username_exists`, err)
			return
		}
		if errors.Is(err, business.ErrUserEmailExists) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgEmailExists), `enterprise.member.create.email_exists`, err)
			return
		}
		if isEnterpriseBadRequest(err) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.member.create.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.member.create.create_user`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    createEnterpriseMemberResponse{UserId: model.UserId},
	})
}

// UpdateEnterpriseMember edits a user's profile and enterprise role.
func UpdateEnterpriseMember(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req updateEnterpriseMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.member.update.bind`, err)
		return
	}
	if err := validateEnterpriseMemberFields(req.Username, req.Name, req.Phone, req.Email); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `enterprise.member.update.validate`, err)
		return
	}

	err := business.UpdateEnterpriseMember(business.UpdateEnterpriseMemberModel{
		AdminId: c.GetInt(`adminId`), OperatorUserId: c.GetInt(`userId`), UserId: cast.ToInt(c.Param(`userId`)), Username: req.Username,
		Name: req.Name, Phone: req.Phone, Email: req.Email, RoleId: req.RoleId, Avatar: req.Avatar,
		DepartmentIds: req.DepartmentIds, UpdateDepartments: true,
	})
	if err != nil {
		switch {
		case errors.Is(err, business.ErrUserUsernameExists):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgUserExist), `enterprise.member.update.username_exists`, err)
		case errors.Is(err, business.ErrUserEmailExists):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgEmailExists), `enterprise.member.update.email_exists`, err)
		case errors.Is(err, business.ErrEnterpriseMemberNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgUserNotFound))
		case errors.Is(err, business.ErrAvatarInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarInvalid), `enterprise.member.update.avatar`, err)
		case isEnterpriseBadRequest(err):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.member.update.business_validate`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.member.update`, err)
		}
		return
	}

	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

func toListEnterpriseMembersResponse(model business.ListEnterpriseMembersModel) []listEnterpriseMembersResponseItem {
	items := make([]listEnterpriseMembersResponseItem, 0, len(model.Items))
	for _, member := range model.Items {
		departmentIds := make([]string, 0, len(member.DepartmentIds))
		for _, id := range member.DepartmentIds {
			departmentIds = append(departmentIds, cast.ToString(id))
		}
		items = append(items, listEnterpriseMembersResponseItem{
			Id:            cast.ToString(member.Id),
			Username:      member.Username,
			Name:          member.Name,
			Phone:         member.Phone,
			Email:         member.Email,
			Avatar:        member.Avatar,
			RoleId:        cast.ToString(member.RoleId),
			Status:        cast.ToString(member.Status),
			IsOwner:       member.IsOwner,
			InvitedByLink: member.InvitationId > 0,
			DepartmentIds: departmentIds,
			CreatedAt:     cast.ToString(member.CreatedAt),
		})
	}
	return items
}

func validateEnterpriseMemberFields(username, name, phone, email string) error {
	if err := validate.ValidateAccount("username", username); err != nil {
		return err
	}
	if err := validate.ValidateProfile(strings.TrimSpace(name)); err != nil {
		return err
	}
	if err := validate.ValidateMemberPhone(strings.TrimSpace(phone)); err != nil {
		return err
	}
	return validate.ValidateMemberEmail(email)
}

// transferTenantOwnerRequest describes the transfer-default-system-administrator request body.
type transferTenantOwnerRequest struct {
	TargetUserId int `json:"target_user_id" binding:"required"`
}

// TransferTenantOwner hands the default system administrator role to another enterprise member.
// Guarded by identity, not by a permission key: only the current owner may call it.
func TransferTenantOwner(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req transferTenantOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.owner.transfer.bind`, err)
		return
	}

	if err := business.TransferTenantOwner(business.TransferTenantOwnerModel{
		AdminId:        c.GetInt(`adminId`),
		OperatorUserId: c.GetInt(`userId`),
		TargetUserId:   req.TargetUserId,
	}); err != nil {
		if errors.Is(err, business.ErrOwnerTransferForbidden) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
		if isEnterpriseBadRequest(err) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgOwnerTransferTargetInvalid), `enterprise.owner.transfer.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.owner.transfer.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// enterpriseMemberRemovalPreviewResponse tells the removal dialog which hand-over rows to show.
type enterpriseMemberRemovalPreviewResponse struct {
	OwnedWorkspaces []ownedWorkspaceResponseItem `json:"owned_workspaces"`
	UnfinishedCount int                          `json:"unfinished_count"`
}

type ownedWorkspaceResponseItem struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

// PreviewEnterpriseMemberRemoval reports what removing a member would drag along.
func PreviewEnterpriseMemberRemoval(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, err := business.PreviewEnterpriseMemberRemoval(business.EnterpriseMemberRemovalPreviewModel{
		AdminId: c.GetInt(`adminId`),
		UserId:  cast.ToInt(c.Param(`userId`)),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.member.remove.preview`, err)
		return
	}

	workspaces := make([]ownedWorkspaceResponseItem, 0, len(model.OwnedWorkspaces))
	for _, workspace := range model.OwnedWorkspaces {
		workspaces = append(workspaces, ownedWorkspaceResponseItem{
			Id:   cast.ToString(workspace.Id),
			Name: workspace.Name,
		})
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: enterpriseMemberRemovalPreviewResponse{
			OwnedWorkspaces: workspaces,
			UnfinishedCount: model.UnfinishedCount,
		},
	})
}

// RemoveEnterpriseMember removes a user from the current tenant.
// Hand-over targets ride along as query parameters because the route is a DELETE.
func RemoveEnterpriseMember(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.Param(`userId`))

	if err := business.RemoveEnterpriseMember(business.RemoveEnterpriseMemberModel{
		AdminId:               adminId,
		OperatorUserId:        c.GetInt(`userId`),
		UserId:                userId,
		TransferOwnerToUserId: cast.ToInt(c.Query(`transferOwnerTo`)),
		TransferItemsToUserId: cast.ToInt(c.Query(`transferItemsTo`)),
	}); err != nil {
		if errors.Is(err, business.ErrOwnerTransferTarget) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgOwnerTransferTargetInvalid), `enterprise.member.remove.transfer_target`, err)
			return
		}
		if isEnterpriseBadRequest(err) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.member.remove.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.member.remove.delete`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// updateEnterpriseMemberStatusRequest describes the enable/disable member request body.
type updateEnterpriseMemberStatusRequest struct {
	Status *int `json:"status" binding:"required"`
}

// UpdateEnterpriseMemberStatus enables or disables a member under the current tenant.
func UpdateEnterpriseMemberStatus(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req updateEnterpriseMemberStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.member.status.bind`, err)
		return
	}

	if err := business.UpdateEnterpriseMemberStatus(business.UpdateEnterpriseMemberStatusModel{
		AdminId:        c.GetInt(`adminId`),
		OperatorUserId: c.GetInt(`userId`),
		UserId:         cast.ToInt(c.Param(`userId`)),
		Status:         *req.Status,
	}); err != nil {
		if errors.Is(err, business.ErrEnterpriseMemberNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgUserNotFound), `enterprise.member.status.not_found`, err)
			return
		}
		if isEnterpriseBadRequest(err) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.member.status.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.member.status.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ListMemberLogs returns the tenant's member operation logs (create/remove/enable/disable).
func ListMemberLogs(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model, err := business.ListMemberLogs(business.ListMemberLogsModel{
		AdminId:  c.GetInt(`adminId`),
		Keyword:  c.Query(`keyword`),
		Page:     cast.ToInt(c.DefaultQuery(`page`, `1`)),
		PageSize: cast.ToInt(c.DefaultQuery(`page_size`, `20`)),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.member.logs.list`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    gin.H{`items`: model.Items, `total`: model.Total},
	})
}

// resetEnterpriseMemberPasswordRequest describes the reset-member-password request body.
type resetEnterpriseMemberPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// ResetEnterpriseMemberPassword resets a user's password under the current tenant.
func ResetEnterpriseMemberPassword(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	operatorUserId := cast.ToInt(c.GetInt(`userId`))
	targetUserId := cast.ToInt(c.Param(`userId`))

	var req resetEnterpriseMemberPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.member.reset_password.bind`, err)
		return
	}

	if err := business.ResetEnterpriseMemberPassword(business.ResetEnterpriseMemberPasswordModel{
		AdminId:        adminId,
		OperatorUserId: operatorUserId,
		TargetUserId:   targetUserId,
		NewPassword:    req.Password,
	}); err != nil {
		if isEnterpriseBadRequest(err) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.member.reset_password.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.member.reset_password.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ListEnterpriseRoles returns all enterprise-level roles for role selection dropdown.
func ListEnterpriseRoles(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))

	roles, err := business.ListEnterpriseRoles(adminId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.role.list.list_enterprise_roles`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListEnterpriseRolesResponse(roles, lang),
	})
}

// listEnterpriseRolesResponseItem is the JSON payload of one enterprise role.
type listEnterpriseRolesResponseItem struct {
	Id                  int                          `json:"id"`
	Name                string                       `json:"name"`
	Description         string                       `json:"description"`
	IsSystem            int                          `json:"is_system"`
	Permissions         []string                     `json:"permissions"`
	MemberCount         int                          `json:"member_count"`
	TeamWorkViewScope   string                       `json:"team_work_view_scope"`
	TeamWorkViewTargets []teamWorkTargetResponseItem `json:"team_work_view_targets"`
}

// teamWorkTargetResponseItem is one entry of a partial team work view scope.
type teamWorkTargetResponseItem struct {
	Type string `json:"type"`
	Id   int    `json:"id"`
}

func toTeamWorkTargetResponseItems(targets []business.TeamWorkTargetModel) []teamWorkTargetResponseItem {
	items := make([]teamWorkTargetResponseItem, 0, len(targets))
	for _, target := range targets {
		items = append(items, teamWorkTargetResponseItem{Type: target.Type, Id: target.Id})
	}
	return items
}

func toListEnterpriseRolesResponse(roles []business.EnterpriseRoleModel, lang string) []listEnterpriseRolesResponseItem {
	items := make([]listEnterpriseRolesResponseItem, 0, len(roles))
	for _, role := range roles {
		description := role.Description
		if description != `` {
			description = lib_i18n.ShowStored(lang, description)
		}
		items = append(items, listEnterpriseRolesResponseItem{
			Id:                  role.Id,
			Name:                lib_i18n.ShowStored(lang, role.Name),
			Description:         description,
			IsSystem:            role.IsSystem,
			Permissions:         role.Permissions,
			MemberCount:         role.MemberCount,
			TeamWorkViewScope:   role.TeamWorkViewScope,
			TeamWorkViewTargets: toTeamWorkTargetResponseItems(role.TeamWorkViewTargets),
		})
	}
	return items
}

// CreateEnterpriseRole creates a custom enterprise role.
func CreateEnterpriseRole(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.create.bind`, err)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == `` || len([]rune(req.Name)) > 20 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.create.validate`, errors.New(`invalid role name`))
		return
	}
	if len([]rune(req.Description)) > 20 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.create.validate`, errors.New(`invalid role description`))
		return
	}

	roleId, err := business.CreateEnterpriseRole(adminId, req.Name, req.Description, req.Permissions)
	if err != nil {
		if isEnterpriseBadRequest(err) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.create.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.role.create.insert`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    map[string]interface{}{"roleId": roleId},
	})
}

// UpdateEnterpriseRolePermissions saves permissions for a custom enterprise role.
func UpdateEnterpriseRolePermissions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	roleId := cast.ToInt(c.Param(`roleId`))

	var req struct {
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.update_permissions.bind`, err)
		return
	}

	if err := business.UpdateEnterpriseRolePermissions(adminId, roleId, req.Permissions); err != nil {
		if isEnterpriseBadRequest(err) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.update_permissions.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.role.update_permissions.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// UpdateEnterpriseRoleBasic updates the name and description of a custom enterprise role.
func UpdateEnterpriseRoleBasic(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	roleId := cast.ToInt(c.Param(`roleId`))

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.update_basic.bind`, err)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == `` || len([]rune(req.Name)) > 20 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.update_basic.validate`, errors.New(`invalid role name`))
		return
	}
	if len([]rune(req.Description)) > 20 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.update_basic.validate`, errors.New(`invalid role description`))
		return
	}

	params := define.UpdateRoleBasicReq{
		AdminID:     adminId,
		RoleID:      roleId,
		Name:        req.Name,
		Description: req.Description,
	}
	if err := business.UpdateEnterpriseRoleBasic(params); err != nil {
		if isEnterpriseBadRequest(err) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.update_basic.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.role.update_basic.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// updateEnterpriseRoleTeamWorkScopeRequest is the JSON body for saving a role's team work scope.
type updateEnterpriseRoleTeamWorkScopeRequest struct {
	ViewScope string                      `json:"viewScope" binding:"required"`
	Targets   []teamWorkTargetRequestItem `json:"targets"`
}

// teamWorkTargetRequestItem is one partial-scope entry sent by the frontend.
type teamWorkTargetRequestItem struct {
	Type string `json:"type"`
	Id   int    `json:"id"`
}

func toTeamWorkTargetModels(items []teamWorkTargetRequestItem) []business.TeamWorkTargetModel {
	targets := make([]business.TeamWorkTargetModel, 0, len(items))
	for _, item := range items {
		targets = append(targets, business.TeamWorkTargetModel{Type: item.Type, Id: item.Id})
	}
	return targets
}

// UpdateEnterpriseRoleTeamWorkScope Save the team work viewing scope of the customized enterprise role.
func UpdateEnterpriseRoleTeamWorkScope(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	roleId := cast.ToInt(c.Param(`roleId`))

	var req updateEnterpriseRoleTeamWorkScopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.update_team_work_scope.bind`, err)
		return
	}

	if err := business.UpdateEnterpriseRoleTeamWorkScope(adminId, roleId, req.ViewScope, toTeamWorkTargetModels(req.Targets)); err != nil {
		if isEnterpriseBadRequest(err) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.update_team_work_scope.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.role.update_team_work_scope.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeleteEnterpriseRole deletes a custom enterprise role.
func DeleteEnterpriseRole(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	roleId := cast.ToInt(c.Param(`roleId`))

	if err := business.DeleteEnterpriseRole(adminId, roleId); err != nil {
		if isEnterpriseBadRequest(err) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.role.delete.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.role.delete.delete`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ListProjectRoles returns all project roles for a workspace (for role config page).
func ListProjectRoles(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId := cast.ToInt(c.GetInt(`adminId`))

	roles, err := business.ListProjectRoles(adminId, wsId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.role.list_project.list_project_roles`, err)
		return
	}
	for i := range roles {
		roles[i].Name = lib_i18n.ShowStored(lang, roles[i].Name)
		if roles[i].Description != `` {
			roles[i].Description = lib_i18n.ShowStored(lang, roles[i].Description)
		}
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    roles,
	})
}

// CreateProjectRole creates a custom project role for a workspace.
func CreateProjectRole(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	wsId := c.Param(`wsId`)

	var req struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.create.bind`, err)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == `` || len([]rune(req.Name)) > 20 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.create.validate`, errors.New(`invalid role name`))
		return
	}
	if len([]rune(req.Description)) > 20 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.create.validate`, errors.New(`invalid role description`))
		return
	}

	roleId, err := business.CreateProjectRole(adminId, wsId, req.Name, req.Description, req.Permissions)
	if err != nil {
		if errors.Is(err, business.ErrProjectRoleNameExists) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.create.name_exists`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `project.role.create.insert`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    map[string]interface{}{"roleId": roleId},
	})
}

// UpdateProjectRolePermissions saves permissions for a project role (custom roles and the editable preset project member role).
func UpdateProjectRolePermissions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	wsId := c.Param(`wsId`)
	roleId := cast.ToInt(c.Param(`roleId`))

	var req struct {
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.update_permissions.bind`, err)
		return
	}

	if err := business.UpdateProjectRolePermissions(adminId, wsId, roleId, req.Permissions); err != nil {
		if errors.Is(err, business.ErrProjectRoleNotFound) || errors.Is(err, business.ErrProjectRoleSystem) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.update_permissions.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `project.role.update_permissions.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// UpdateProjectRoleDataViewScope saves the list data view scope of a project role (custom roles and the editable preset project member role).
func UpdateProjectRoleDataViewScope(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	wsId := c.Param(`wsId`)
	roleId := cast.ToInt(c.Param(`roleId`))

	var req struct {
		ViewScope string `json:"viewScope" binding:"required"`
		MemberIds []int  `json:"memberIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.update_data_view_scope.bind`, err)
		return
	}

	if err := business.UpdateProjectRoleDataViewScope(adminId, wsId, roleId, req.ViewScope, req.MemberIds); err != nil {
		if errors.Is(err, business.ErrProjectRoleNotFound) ||
			errors.Is(err, business.ErrProjectRoleSystem) ||
			errors.Is(err, business.ErrProjectRoleDataViewScopeInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.update_data_view_scope.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `project.role.update_data_view_scope.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// UpdateProjectRoleBasic updates the name and description of a project role.
func UpdateProjectRoleBasic(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	wsId := c.Param(`wsId`)
	roleId := cast.ToInt(c.Param(`roleId`))

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.update_basic.bind`, err)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == `` || len([]rune(req.Name)) > 20 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.update_basic.validate`, errors.New(`invalid role name`))
		return
	}
	if len([]rune(req.Description)) > 20 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.update_basic.validate`, errors.New(`invalid role description`))
		return
	}

	params := define.UpdateRoleBasicReq{
		AdminID:     adminId,
		WorkspaceID: wsId,
		RoleID:      roleId,
		Name:        req.Name,
		Description: req.Description,
	}
	if err := business.UpdateProjectRoleBasic(params); err != nil {
		if errors.Is(err, business.ErrProjectRoleNotFound) ||
			errors.Is(err, business.ErrProjectRoleSystem) ||
			errors.Is(err, business.ErrProjectRoleNameExists) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.update_basic.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `project.role.update_basic.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeleteProjectRole deletes a custom project role.
func DeleteProjectRole(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	wsId := c.Param(`wsId`)
	roleId := cast.ToInt(c.Param(`roleId`))

	if err := business.DeleteProjectRole(adminId, wsId, roleId); err != nil {
		if errors.Is(err, business.ErrProjectRoleNotFound) ||
			errors.Is(err, business.ErrProjectRoleSystem) ||
			errors.Is(err, business.ErrProjectRoleInUse) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.delete.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `project.role.delete.delete`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

func isEnterpriseBadRequest(err error) bool {
	return errors.Is(err, business.ErrEnterpriseAdminProtected) ||
		errors.Is(err, business.ErrOwnerTransferTarget) ||
		errors.Is(err, business.ErrEnterpriseRoleNotFound) ||
		errors.Is(err, business.ErrEnterpriseRoleSystem) ||
		errors.Is(err, business.ErrEnterpriseRoleInUse) ||
		errors.Is(err, business.ErrEnterpriseRoleNameExists) ||
		errors.Is(err, business.ErrEnterpriseTeamWorkScopeInvalid) ||
		errors.Is(err, business.ErrMemberSelfStatusProtected) ||
		errors.Is(err, business.ErrMemberStatusInvalid) ||
		errors.Is(err, business.ErrDepartmentInvalid) ||
		errors.Is(err, business.ErrDepartmentNotFound)
}
