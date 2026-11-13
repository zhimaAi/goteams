// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"net/http"
	"slices"
	"strconv"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

// ======================== Workspace Metadata DTO ========================
// Note: workspace / members / versions historically returned the raw DB Map with string values;
// this response struct keeps the same string types to preserve the JSON contract.

type workspaceMetadataStatusResponse struct {
	Id        int    `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	IsDefault int    `json:"is_default"`
}

type workspaceMetadataPriorityResponse struct {
	Id        int    `json:"id"`
	Key       string `json:"key"`
	Label     string `json:"label"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	IsDefault int    `json:"is_default"`
}

type workspaceMetadataFieldConfigResponse struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Enabled   int    `json:"enabled"`
	Editable  int    `json:"editable"`
	Locked    int    `json:"locked"`
	SortOrder int    `json:"sort_order"`
}

type workspaceMetadataCustomFieldOptionResponse struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type workspaceMetadataCustomFieldCascaderResponse struct {
	Key      string                                         `json:"key"`
	Name     string                                         `json:"name"`
	Children []workspaceMetadataCustomFieldCascaderResponse `json:"children,omitempty"`
}

type workspaceMetadataCustomFieldResponse struct {
	Id          int                                            `json:"id"`
	Key         string                                         `json:"key"`
	Name        string                                         `json:"name"`
	FieldType   string                                         `json:"field_type"`
	IsRequired  int                                            `json:"is_required"`
	SortOrder   int                                            `json:"sort_order"`
	IsEnabled   int                                            `json:"is_enabled"`
	Options     []workspaceMetadataCustomFieldOptionResponse   `json:"options"`
	IntegerOnly int                                            `json:"integer_only"`
	MemberScope string                                         `json:"member_scope,omitempty"`
	RoleIds     []int                                          `json:"role_ids,omitempty"`
	Cascader    []workspaceMetadataCustomFieldCascaderResponse `json:"cascader,omitempty"`
}

type workspaceMetadataGroupResponse struct {
	Statuses     []workspaceMetadataStatusResponse      `json:"statuses"`
	Priorities   []workspaceMetadataPriorityResponse    `json:"priorities"`
	FieldConfigs []workspaceMetadataFieldConfigResponse `json:"field_configs"`
	CustomFields []workspaceMetadataCustomFieldResponse `json:"custom_fields"`
}

type workspaceMetadataStatusTransitionFieldResponse struct {
	Key      string `json:"key"`
	Required int    `json:"required"`
}

type workspaceMetadataStatusTransitionFormResponse struct {
	Enabled int                                              `json:"enabled"`
	Fields  []workspaceMetadataStatusTransitionFieldResponse `json:"fields"`
}

type workspaceMetadataSettingsResponse struct {
	SLAEnabled                    bool                                          `json:"sla_enabled"`
	SLAActivatedAt                int64                                         `json:"sla_activated_at"`
	RequirementInitialStatusId    int                                           `json:"requirement_initial_status_id"`
	DefectInitialStatusId         int                                           `json:"defect_initial_status_id"`
	PlanningInitialStatusId       int                                           `json:"planning_initial_status_id"`
	RequirementFinalStatusIds     []int                                         `json:"requirement_final_status_ids"`
	RequirementCompletedStatusIds []int                                         `json:"requirement_completed_status_ids"`
	RequirementClosedStatusIds    []int                                         `json:"requirement_closed_status_ids"`
	DefectFinalStatusIds          []int                                         `json:"defect_final_status_ids"`
	DefectCompletedStatusIds      []int                                         `json:"defect_completed_status_ids"`
	DefectClosedStatusIds         []int                                         `json:"defect_closed_status_ids"`
	PlanningFinalStatusIds        []int                                         `json:"planning_final_status_ids"`
	RequirementStatusTransition   workspaceMetadataStatusTransitionFormResponse `json:"requirement_status_transition"`
	DefectStatusTransition        workspaceMetadataStatusTransitionFormResponse `json:"defect_status_transition"`
	RequirementAgingControl       agingControlFormResponse                      `json:"requirement_aging_control"`
}

type workspaceMetadataWorkspaceResponse struct {
	Id              string `json:"id"`
	WorkspaceType   string `json:"workspace_type"`
	AdminId         string `json:"admin_id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	Description     string `json:"description"`
	Color           string `json:"color"`
	Icon            string `json:"icon"`
	EndedAt         string `json:"ended_at"`
	ActiveVersionId string `json:"active_version_id"`
	CreatedBy       string `json:"created_by"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

func toWorkspaceMetadataCascaderResponse(nodes []business.CustomFieldCascaderNode) []workspaceMetadataCustomFieldCascaderResponse {
	if len(nodes) == 0 {
		return nil
	}
	result := make([]workspaceMetadataCustomFieldCascaderResponse, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, workspaceMetadataCustomFieldCascaderResponse{
			Key: node.Key, Name: node.Name, Children: toWorkspaceMetadataCascaderResponse(node.Children),
		})
	}
	return result
}

type workspaceMetadataMemberResponse struct {
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
	UserAvatar       string `json:"user_avatar"`
	UserStatus       string `json:"user_status"`
	RoleName         string `json:"role_name"`
	RoleDescription  string `json:"role_description"`
	RoleIsSystem     string `json:"role_is_system"`
	Removed          bool   `json:"removed"`
}

type workspaceMetadataCustomFieldMemberResponse struct {
	UserId            string `json:"user_id"`
	EnterpriseRoleId  string `json:"enterprise_role_id"`
	UserName          string `json:"user_name"`
	UserUsername      string `json:"user_username"`
	UserAvatar        string `json:"user_avatar"`
	UserStatus        string `json:"user_status"`
	IsWorkspaceMember bool   `json:"is_workspace_member"`
	Removed           bool   `json:"removed"`
}

type workspaceMetadataVersionWorkspaceResponse struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	IsOrigin bool   `json:"is_origin"`
}

type workspaceMetadataVersionResponse struct {
	Id               int                                         `json:"id"`
	AdminId          int                                         `json:"admin_id"`
	WorkspaceId      int                                         `json:"workspace_id"`
	Name             string                                      `json:"name"`
	StartDate        int64                                       `json:"start_date"`
	EndDate          int64                                       `json:"end_date"`
	EndedAt          int64                                       `json:"ended_at"`
	IsEnded          bool                                        `json:"is_ended"`
	IterationType    string                                      `json:"iteration_type"`
	Workspaces       []workspaceMetadataVersionWorkspaceResponse `json:"workspaces"`
	RequirementCount int                                         `json:"requirement_count"`
	CompletedCount   int                                         `json:"completed_count"`
	CreatedAt        int64                                       `json:"created_at"`
	UpdatedAt        int64                                       `json:"updated_at"`
}

type workspaceMetadataResponse struct {
	Requirement         workspaceMetadataGroupResponse               `json:"requirement"`
	Defect              workspaceMetadataGroupResponse               `json:"defect"`
	Planning            workspaceMetadataGroupResponse               `json:"planning"`
	Settings            workspaceMetadataSettingsResponse            `json:"settings"`
	Workspace           workspaceMetadataWorkspaceResponse           `json:"workspace"`
	Members             []workspaceMetadataMemberResponse            `json:"members"`
	CustomFieldMembers  []workspaceMetadataCustomFieldMemberResponse `json:"custom_field_members"`
	Versions            []workspaceMetadataVersionResponse           `json:"versions"`
	Permissions         []string                                     `json:"permissions"`
	EnabledApplications []string                                     `json:"enabled_applications"`
}

// GetWorkspaceMetadata returns the workspace's basic metadata.
func GetWorkspaceMetadata(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)

	permissions := business.GetUserPermissions(adminId, userId, wsId)
	if permissions == nil {
		permissions = []string{}
	}
	metadata, err := business.GetWorkspaceMetadata(business.WorkspaceMetadataModel{
		AdminId:         adminId,
		WorkspaceId:     wsId,
		Lang:            lang,
		IncludeVersions: slices.Contains(permissions, `version:view`),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace_metadata.get`, err)
		return
	}
	if metadata.Workspace == nil {
		c.JSON(http.StatusNotFound, define.Response{
			Code:    http.StatusNotFound,
			Message: lib_i18n.Show(lang, define.MsgWorkspaceNotFound),
		})
		return
	}
	metadata.Permissions = permissions
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toWorkspaceMetadataResponse(metadata, lang),
	})
}

// toWorkspaceMetadataResponse converts the metadata model to the response.
func toWorkspaceMetadataResponse(model business.WorkspaceMetadataModel, lang string) workspaceMetadataResponse {
	workspace := model.Workspace
	return workspaceMetadataResponse{
		Requirement: toWorkspaceMetadataGroupResponse(model.Requirement),
		Defect:      toWorkspaceMetadataGroupResponse(model.Defect),
		Planning:    toWorkspaceMetadataGroupResponse(model.Planning),
		Settings: workspaceMetadataSettingsResponse{
			RequirementInitialStatusId:    model.Settings.RequirementInitialStatusId,
			DefectInitialStatusId:         model.Settings.DefectInitialStatusId,
			PlanningInitialStatusId:       model.Settings.PlanningInitialStatusId,
			RequirementFinalStatusIds:     model.Settings.RequirementFinalStatusIds,
			RequirementCompletedStatusIds: model.Settings.RequirementCompletedStatusIds,
			RequirementClosedStatusIds:    model.Settings.RequirementClosedStatusIds,
			DefectFinalStatusIds:          model.Settings.DefectFinalStatusIds,
			DefectCompletedStatusIds:      model.Settings.DefectCompletedStatusIds,
			DefectClosedStatusIds:         model.Settings.DefectClosedStatusIds,
			PlanningFinalStatusIds:        model.Settings.PlanningFinalStatusIds,
			RequirementStatusTransition:   toWorkspaceMetadataStatusTransitionFormResponse(model.Settings.RequirementStatusTransition),
			DefectStatusTransition:        toWorkspaceMetadataStatusTransitionFormResponse(model.Settings.DefectStatusTransition),
			RequirementAgingControl:       toAgingControlFormResponse(model.Settings.RequirementAgingControl),
			SLAEnabled:                    model.Settings.SLAEnabled,
			SLAActivatedAt:                model.Settings.SLAActivatedAt,
		},
		Workspace: workspaceMetadataWorkspaceResponse{
			WorkspaceType: workspace.WorkspaceType,
			Id:            strconv.Itoa(workspace.Id), AdminId: strconv.Itoa(workspace.AdminId), Name: workspace.Name, Slug: workspace.Slug,
			Description: workspace.Description, Color: workspace.Color, Icon: workspace.Icon,
			EndedAt:         strconv.FormatInt(workspace.EndedAt, 10),
			ActiveVersionId: strconv.Itoa(workspace.ActiveVersionId), CreatedBy: strconv.Itoa(workspace.CreatedBy),
			CreatedAt: strconv.FormatInt(workspace.CreatedAt, 10), UpdatedAt: strconv.FormatInt(workspace.UpdatedAt, 10),
		},
		Members:             toWorkspaceMetadataMembersResponse(model.Members, lang),
		CustomFieldMembers:  toWorkspaceMetadataCustomFieldMembersResponse(model.CustomFieldMembers),
		Versions:            toWorkspaceMetadataVersionsResponse(model.Versions),
		Permissions:         model.Permissions,
		EnabledApplications: model.EnabledApplications,
	}
}

func toWorkspaceMetadataStatusTransitionFormResponse(model business.StatusTransitionFormModel) workspaceMetadataStatusTransitionFormResponse {
	fields := make([]workspaceMetadataStatusTransitionFieldResponse, 0, len(model.Fields))
	for _, item := range model.Fields {
		fields = append(fields, workspaceMetadataStatusTransitionFieldResponse{Key: item.Key, Required: item.Required})
	}
	return workspaceMetadataStatusTransitionFormResponse{Enabled: model.Enabled, Fields: fields}
}

// toWorkspaceMetadataGroupResponse converts a single work-item type's metadata group.
func toWorkspaceMetadataGroupResponse(model business.WorkspaceMetadataGroupModel) workspaceMetadataGroupResponse {
	response := workspaceMetadataGroupResponse{
		Statuses:     make([]workspaceMetadataStatusResponse, 0, len(model.Statuses)),
		Priorities:   make([]workspaceMetadataPriorityResponse, 0, len(model.Priorities)),
		FieldConfigs: make([]workspaceMetadataFieldConfigResponse, 0, len(model.FieldConfigs)),
		CustomFields: make([]workspaceMetadataCustomFieldResponse, 0, len(model.CustomFields)),
	}
	for _, item := range model.Statuses {
		response.Statuses = append(response.Statuses, workspaceMetadataStatusResponse(item))
	}
	for _, item := range model.Priorities {
		response.Priorities = append(response.Priorities, workspaceMetadataPriorityResponse(item))
	}
	for _, item := range model.FieldConfigs {
		response.FieldConfigs = append(response.FieldConfigs, workspaceMetadataFieldConfigResponse(item))
	}
	for _, item := range model.CustomFields {
		options := make([]workspaceMetadataCustomFieldOptionResponse, 0, len(item.Options))
		for _, option := range item.Options {
			options = append(options, workspaceMetadataCustomFieldOptionResponse(option))
		}
		// Keep null when options are empty, consistent with the existing contract.
		if item.Options == nil {
			options = nil
		}
		response.CustomFields = append(response.CustomFields, workspaceMetadataCustomFieldResponse{
			Id: item.Id, Key: item.Key, Name: item.Name, FieldType: item.FieldType, IsRequired: item.IsRequired,
			SortOrder: item.SortOrder, IsEnabled: item.IsEnabled, Options: options,
			IntegerOnly: item.IntegerOnly, MemberScope: item.MemberScope, RoleIds: item.RoleIds,
			Cascader: toWorkspaceMetadataCascaderResponse(item.Cascader),
		})
	}
	return response
}

// toWorkspaceMetadataMembersResponse converts the member list, translating role names by language.
func toWorkspaceMetadataMembersResponse(models []business.WorkspaceMemberModel, lang string) []workspaceMetadataMemberResponse {
	items := make([]workspaceMetadataMemberResponse, 0, len(models))
	for _, member := range models {
		roleName, roleDescription := member.RoleName, member.RoleDescription
		if roleName != `` {
			roleName = lib_i18n.ShowStored(lang, roleName)
		}
		if roleDescription != `` {
			roleDescription = lib_i18n.ShowStored(lang, roleDescription)
		}
		items = append(items, workspaceMetadataMemberResponse{
			Id: strconv.Itoa(member.Id), AdminId: strconv.Itoa(member.AdminId), WorkspaceId: strconv.Itoa(member.WorkspaceId),
			UserId: strconv.Itoa(member.UserId), RoleId: strconv.Itoa(member.RoleId),
			EnterpriseRoleId: strconv.Itoa(member.EnterpriseRoleId), MemberType: member.MemberType,
			JoinedAt: strconv.FormatInt(member.JoinedAt, 10), CreatedAt: strconv.FormatInt(member.CreatedAt, 10),
			UpdatedAt: strconv.FormatInt(member.UpdatedAt, 10), UserName: member.UserName,
			UserUsername: member.UserUsername, UserAvatar: member.UserAvatar,
			UserStatus: strconv.Itoa(member.UserStatus),
			RoleName:   roleName, RoleDescription: roleDescription, RoleIsSystem: strconv.Itoa(member.RoleIsSystem),
			Removed: member.Removed,
		})
	}
	return items
}

func toWorkspaceMetadataCustomFieldMembersResponse(models []business.WorkspaceMetadataCustomFieldMemberModel) []workspaceMetadataCustomFieldMemberResponse {
	items := make([]workspaceMetadataCustomFieldMemberResponse, 0, len(models))
	for _, member := range models {
		items = append(items, workspaceMetadataCustomFieldMemberResponse{
			UserId: strconv.Itoa(member.UserId), EnterpriseRoleId: strconv.Itoa(member.EnterpriseRoleId),
			UserName: member.UserName, UserUsername: member.UserUsername, UserAvatar: member.UserAvatar,
			UserStatus: strconv.Itoa(member.UserStatus), IsWorkspaceMember: member.IsWorkspaceMember, Removed: member.Removed,
		})
	}
	return items
}

// toWorkspaceMetadataVersionsResponse converts the iteration list.
func toWorkspaceMetadataVersionsResponse(models []business.VersionModel) []workspaceMetadataVersionResponse {
	items := make([]workspaceMetadataVersionResponse, 0, len(models))
	for _, version := range models {
		workspaces := make([]workspaceMetadataVersionWorkspaceResponse, 0, len(version.Workspaces))
		for _, workspace := range version.Workspaces {
			workspaces = append(workspaces, workspaceMetadataVersionWorkspaceResponse{
				Id: workspace.Id, Name: workspace.Name, IsOrigin: workspace.IsOrigin,
			})
		}
		items = append(items, workspaceMetadataVersionResponse{
			Id: version.Id, AdminId: version.AdminId, WorkspaceId: version.WorkspaceId,
			Name: version.Name, StartDate: version.StartDate, EndDate: version.EndDate,
			EndedAt: version.EndedAt, IsEnded: version.EndedAt > 0, IterationType: version.IterationType(), Workspaces: workspaces,
			RequirementCount: version.RequirementCount, CompletedCount: version.CompletedCount,
			CreatedAt: version.CreatedAt, UpdatedAt: version.UpdatedAt,
		})
	}
	return items
}
