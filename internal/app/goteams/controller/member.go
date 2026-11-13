// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

type listWorkspaceMembersResponseItem struct {
	Id               string `json:"id"`
	AdminId          string `json:"admin_id"`
	WorkspaceId      string `json:"workspace_id"`
	UserId           string `json:"user_id"`
	RoleId           string `json:"role_id"`
	EnterpriseRoleId string `json:"enterprise_role_id"`
	MemberType       string `json:"member_type"`
	JoinedAt         string `json:"joined_at"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
	UserName         string `json:"user_name"`
	UserUsername     string `json:"user_username"`
	UserPhone        string `json:"user_phone"`
	UserRemark       string `json:"user_remark"`
	UserAvatar       string `json:"user_avatar"`
	UserStatus       string `json:"user_status"`
	RoleName         string `json:"role_name"`
	RoleDescription  string `json:"role_description"`
	RoleIsSystem     string `json:"role_is_system"`
	IsOwner          bool   `json:"is_owner"`
}

type memberCandidateDepartmentItem struct {
	Id       string `json:"id"`
	ParentId string `json:"parent_id"`
	Name     string `json:"name"`
}

type memberCandidateRoleItem struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type memberCandidateItem struct {
	Id            string   `json:"id"`
	Username      string   `json:"username"`
	Name          string   `json:"name"`
	Avatar        string   `json:"avatar"`
	RoleId        string   `json:"role_id"`
	DepartmentIds []string `json:"department_ids"`
	Joined        bool     `json:"joined"`
}

type listMemberCandidatesResponse struct {
	EnterpriseName  string                          `json:"enterprise_name"`
	Departments     []memberCandidateDepartmentItem `json:"departments"`
	EnterpriseRoles []memberCandidateRoleItem       `json:"enterprise_roles"`
	Members         []memberCandidateItem           `json:"members"`
}

type copyProjectMembersRequest struct {
	SourceWsId int   `json:"sourceWsId" binding:"required,gt=0"`
	RoleIds    []int `json:"roleIds" binding:"required,min=1"`
}

type copyProjectMembersResponse struct {
	CreatedRoles int `json:"created_roles"`
	AddedMembers int `json:"added_members"`
}

// ======================== Member Management ========================

// ListMembers returns the member list for a workspace.
func ListMembers(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, err := business.ListMembersByWorkspace(business.ListWorkspaceMembersModel{
		AdminId:     c.GetInt(`adminId`),
		WorkspaceId: c.Param(`wsId`),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `member.list.list_members_by_workspace`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListWorkspaceMembersResponse(model, lang),
	})
}

// GetMemberAvatar returns a workspace member's avatar file (caller must be a member of the workspace; isolated by admin_id).
// Frontend avatars are local disk paths that cannot be served as static resources, so they are returned per user via a session-authenticated blob endpoint.
func GetMemberAvatar(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	userId := cast.ToInt(c.Param(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	if !business.IsWorkspaceMember(adminId, userId, wsId) {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgAvatarNotFound))
		return
	}
	path, err := business.GetUserAvatarPath(adminId, userId)
	if err != nil {
		if errors.Is(err, business.ErrAvatarNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgAvatarNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `member.avatar.get_path`, err)
		return
	}
	c.Header(`Cache-Control`, `no-store`)
	if err := lib_storage.M.Serve(path, c); err != nil {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgAvatarNotFound))
	}
}

// ListMemberCandidates returns the organization tree source data (enterprise name, departments,
// enterprise roles) and every enterprise member flagged with its join state in this workspace.
func ListMemberCandidates(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, err := business.ListMemberCandidates(business.ListMemberCandidatesModel{
		AdminId:     c.GetInt(`adminId`),
		WorkspaceId: c.Param(`wsId`),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `member.candidates.list`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListMemberCandidatesResponse(model, lang),
	})
}

// CopyProjectMembers copies the selected roles and their members from another project into this one.
func CopyProjectMembers(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req copyProjectMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `member.copy.bind`, err)
		return
	}

	model, err := business.CopyProjectMembers(business.CopyProjectMembersModel{
		AdminId:           c.GetInt(`adminId`),
		UserId:            c.GetInt(`userId`),
		WorkspaceId:       c.Param(`wsId`),
		SourceWorkspaceId: cast.ToString(req.SourceWsId),
		RoleIds:           req.RoleIds,
	})
	if err != nil {
		switch {
		case errors.Is(err, business.ErrCopySourceWorkspaceInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkspaceNotFound), `member.copy.source_invalid`, err)
		case errors.Is(err, business.ErrCopySourceWorkspaceDenied):
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		case errors.Is(err, business.ErrProjectRoleNotFound), errors.Is(err, business.ErrProjectRoleNameExists):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkspaceRoleNotFound), `member.copy.role_invalid`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `member.copy.copy_project_members`, err)
		}
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    copyProjectMembersResponse{CreatedRoles: model.CreatedRoles, AddedMembers: model.AddedMembers},
	})
}

// AddMember adds one or more enterprise members to a workspace.
func AddMember(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req struct {
		UserIds []int `json:"userIds" binding:"required"`
		RoleId  int   `json:"roleId" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `member.add.bind`, err)
		return
	}

	added, err := business.AddMembersToWorkspace(adminId, wsId, req.UserIds, req.RoleId, c.GetInt(`userId`))
	if err != nil {
		if msgKey, ok := memberBadRequestMessage(err); ok {
			abortBadRequest(c, lib_i18n.Show(lang, msgKey), `member.add.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `member.add.add_members`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    map[string]int{`added`: added},
	})
}

// UpdateMemberRole changes a workspace member's project role.
func UpdateMemberRole(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	userId := cast.ToInt(c.Param(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req struct {
		RoleId int `json:"roleId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `member.update_role.bind`, err)
		return
	}

	if err := business.UpdateMemberRole(adminId, wsId, userId, cast.ToString(req.RoleId)); err != nil {
		if msgKey, ok := memberBadRequestMessage(err); ok {
			abortBadRequest(c, lib_i18n.Show(lang, msgKey), `member.update_role.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `member.update_role.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// transferWorkspaceOwnerRequest describes the transfer-default-project-admin request body.
type transferWorkspaceOwnerRequest struct {
	TargetUserId int `json:"target_user_id" binding:"required"`
}

// TransferWorkspaceOwner hands the default project admin role to another project member.
// Guarded by identity inside business: the current project owner, or the tenant owner as fallback.
func TransferWorkspaceOwner(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req transferWorkspaceOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `member.owner.transfer.bind`, err)
		return
	}

	if err := business.TransferWorkspaceOwner(business.TransferWorkspaceOwnerModel{
		AdminId:        cast.ToInt(c.GetInt(`adminId`)),
		WorkspaceId:    cast.ToInt(c.Param(`wsId`)),
		OperatorUserId: c.GetInt(`userId`),
		TargetUserId:   req.TargetUserId,
	}); err != nil {
		switch {
		case errors.Is(err, business.ErrOwnerTransferForbidden):
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		case errors.Is(err, business.ErrOwnerTransferTarget), errors.Is(err, business.ErrWorkspaceRoleNotFound):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgOwnerTransferTargetInvalid), `member.owner.transfer.validate`, err)
		case errors.Is(err, business.ErrWorkspaceNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceNotFound))
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `member.owner.transfer.update`, err)
		}
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// workspaceMemberRemovalPreviewResponse tells the project removal dialog whether to offer a hand-over.
type workspaceMemberRemovalPreviewResponse struct {
	UnfinishedCount int `json:"unfinished_count"`
}

// PreviewWorkspaceMemberRemoval reports how many unfinished work items the member holds here.
func PreviewWorkspaceMemberRemoval(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, err := business.PreviewWorkspaceMemberRemoval(business.WorkspaceMemberRemovalPreviewModel{
		AdminId:     cast.ToInt(c.GetInt(`adminId`)),
		WorkspaceId: cast.ToInt(c.Param(`wsId`)),
		UserId:      cast.ToInt(c.Param(`userId`)),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `member.remove.preview`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    workspaceMemberRemovalPreviewResponse{UnfinishedCount: model.UnfinishedCount},
	})
}

// RemoveMember removes a member from a workspace.
// The hand-over target rides along as a query parameter because the route is a DELETE.
func RemoveMember(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))

	if err := business.RemoveMemberFromWorkspace(business.RemoveWorkspaceMemberModel{
		AdminId:               adminId,
		WorkspaceId:           cast.ToInt(c.Param(`wsId`)),
		OperatorUserId:        c.GetInt(`userId`),
		UserId:                cast.ToInt(c.Param(`userId`)),
		TransferItemsToUserId: cast.ToInt(c.Query(`transferItemsTo`)),
	}); err != nil {
		switch {
		case errors.Is(err, business.ErrProjectOwnerProtected):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgProjectOwnerProtected), `member.remove.owner_protected`, err)
		case errors.Is(err, business.ErrOwnerTransferTarget):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgOwnerTransferTargetInvalid), `member.remove.transfer_target`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `member.remove.remove_member_from_workspace`, err)
		}
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

func toListWorkspaceMembersResponse(model business.ListWorkspaceMembersModel, lang string) []listWorkspaceMembersResponseItem {
	items := make([]listWorkspaceMembersResponseItem, 0, len(model.Items))
	for _, member := range model.Items {
		roleName, roleDescription := member.RoleName, member.RoleDescription
		if roleName != `` {
			roleName = lib_i18n.ShowStored(lang, roleName)
		}
		if roleDescription != `` {
			roleDescription = lib_i18n.ShowStored(lang, roleDescription)
		}
		items = append(items, listWorkspaceMembersResponseItem{
			Id: cast.ToString(member.Id), AdminId: cast.ToString(member.AdminId), WorkspaceId: cast.ToString(member.WorkspaceId),
			UserId: cast.ToString(member.UserId), RoleId: cast.ToString(member.RoleId),
			EnterpriseRoleId: cast.ToString(member.EnterpriseRoleId), MemberType: member.MemberType,
			JoinedAt: cast.ToString(member.JoinedAt), CreatedAt: cast.ToString(member.CreatedAt), UpdatedAt: cast.ToString(member.UpdatedAt),
			UserName: member.UserName, UserUsername: member.UserUsername, UserPhone: member.UserPhone, UserRemark: member.UserRemark, UserAvatar: member.UserAvatar,
			UserStatus: cast.ToString(member.UserStatus),
			RoleName:   roleName, RoleDescription: roleDescription, RoleIsSystem: cast.ToString(member.RoleIsSystem),
			IsOwner: member.IsOwner,
		})
	}
	return items
}

func toListMemberCandidatesResponse(model business.ListMemberCandidatesModel, lang string) listMemberCandidatesResponse {
	response := listMemberCandidatesResponse{
		EnterpriseName:  model.EnterpriseName,
		Departments:     make([]memberCandidateDepartmentItem, 0, len(model.Departments)),
		EnterpriseRoles: make([]memberCandidateRoleItem, 0, len(model.EnterpriseRoles)),
		Members:         make([]memberCandidateItem, 0, len(model.Items)),
	}
	for _, department := range model.Departments {
		response.Departments = append(response.Departments, memberCandidateDepartmentItem{
			Id: cast.ToString(department.Id), ParentId: cast.ToString(department.ParentId), Name: department.Name,
		})
	}
	for _, role := range model.EnterpriseRoles {
		response.EnterpriseRoles = append(response.EnterpriseRoles, memberCandidateRoleItem{
			Id: cast.ToString(role.Id), Name: lib_i18n.ShowStored(lang, role.Name),
		})
	}
	for _, member := range model.Items {
		departmentIds := make([]string, 0, len(member.DepartmentIds))
		for _, id := range member.DepartmentIds {
			departmentIds = append(departmentIds, cast.ToString(id))
		}
		response.Members = append(response.Members, memberCandidateItem{
			Id: cast.ToString(member.Id), Username: member.Username, Name: member.Name, Avatar: member.Avatar,
			RoleId: cast.ToString(member.RoleId), DepartmentIds: departmentIds, Joined: member.Joined,
		})
	}
	return response
}

func translateMemberRoleFields(member map[string]string, lang string) {
	if member[`role_name`] != `` {
		member[`role_name`] = lib_i18n.ShowStored(lang, member[`role_name`])
	}
	if member[`role_description`] != `` {
		member[`role_description`] = lib_i18n.ShowStored(lang, member[`role_description`])
	}
}

func memberBadRequestMessage(err error) (string, bool) {
	if errors.Is(err, business.ErrWorkspaceMemberCandidateInvalid) {
		return define.MsgUserNotFound, true
	}
	if errors.Is(err, business.ErrWorkspaceMemberAlreadyExists) {
		return define.MsgWorkspaceMemberExist, true
	}
	if errors.Is(err, business.ErrWorkspaceRoleNotFound) {
		return define.MsgWorkspaceRoleNotFound, true
	}
	return ``, false
}
