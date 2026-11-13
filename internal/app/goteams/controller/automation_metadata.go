package controller

import (
	"strconv"

	"goteams/internal/app/goteams/business"
)

type automationMetadataQuery struct {
	RuleId string `form:"rule_id"`
	Type   string `form:"type"`
}

type automationMetadataFieldOptionResponse struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type automationMetadataFieldResponse struct {
	MemberScope string                                  `json:"member_scope,omitempty"`
	RoleIds     []int                                   `json:"role_ids,omitempty"`
	Key         string                                  `json:"key"`
	Label       string                                  `json:"label"`
	Type        string                                  `json:"type"`
	Editable    bool                                    `json:"editable"`
	Deleted     bool                                    `json:"deleted,omitempty"`
	Options     []automationMetadataFieldOptionResponse `json:"options,omitempty"`
}

type automationMetadataVariableResponse struct {
	Key   string `json:"key"`
	Group string `json:"group"`
	Label string `json:"label"`
}

type automationMetadataCopyTargetResponse struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type automationMetadataStatusResponse struct {
	Id        int    `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	IsDefault int    `json:"is_default"`
}

type automationMetadataPriorityResponse struct {
	Id        int    `json:"id"`
	Key       string `json:"key"`
	Label     string `json:"label"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	IsDefault int    `json:"is_default"`
}

type automationMetadataFieldConfigResponse struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Enabled   int    `json:"enabled"`
	Editable  int    `json:"editable"`
	Locked    int    `json:"locked"`
	SortOrder int    `json:"sort_order"`
}

type automationMetadataCustomFieldOptionResponse struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type automationMetadataCustomFieldResponse struct {
	Id         int                                           `json:"id"`
	Key        string                                        `json:"key"`
	Name       string                                        `json:"name"`
	FieldType  string                                        `json:"field_type"`
	IsRequired int                                           `json:"is_required"`
	SortOrder  int                                           `json:"sort_order"`
	IsEnabled  int                                           `json:"is_enabled"`
	Options    []automationMetadataCustomFieldOptionResponse `json:"options"`
}

type automationMetadataGroupResponse struct {
	Statuses     []automationMetadataStatusResponse      `json:"statuses"`
	Priorities   []automationMetadataPriorityResponse    `json:"priorities"`
	FieldConfigs []automationMetadataFieldConfigResponse `json:"field_configs"`
	CustomFields []automationMetadataCustomFieldResponse `json:"custom_fields"`
}

type automationMetadataSettingsResponse struct {
	RequirementInitialStatusId    int   `json:"requirement_initial_status_id"`
	DefectInitialStatusId         int   `json:"defect_initial_status_id"`
	RequirementFinalStatusIds     []int `json:"requirement_final_status_ids"`
	RequirementCompletedStatusIds []int `json:"requirement_completed_status_ids"`
	RequirementClosedStatusIds    []int `json:"requirement_closed_status_ids"`
	DefectFinalStatusIds          []int `json:"defect_final_status_ids"`
	DefectCompletedStatusIds      []int `json:"defect_completed_status_ids"`
	DefectClosedStatusIds         []int `json:"defect_closed_status_ids"`
}

type automationMetadataWorkspaceResponse struct {
	Id              string `json:"id"`
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

type automationMetadataMemberPushResponse struct {
	DingtalkAtMobiles        string `json:"dingtalk_at_mobiles"`
	DingtalkAtUserIds        string `json:"dingtalk_at_user_ids"`
	FeishuUserId             string `json:"feishu_user_id"`
	WecomMentionedList       string `json:"wecom_mentioned_list"`
	WecomMentionedMobileList string `json:"wecom_mentioned_mobile_list"`
}

type automationMetadataMemberResponse struct {
	Id              string                               `json:"id"`
	AdminId         string                               `json:"admin_id"`
	WorkspaceId     string                               `json:"workspace_id"`
	UserId          string                               `json:"user_id"`
	RoleId          string                               `json:"role_id"`
	MemberType      string                               `json:"member_type"`
	JoinedAt        string                               `json:"joined_at"`
	CreatedAt       string                               `json:"created_at"`
	UpdatedAt       string                               `json:"updated_at"`
	UserName        string                               `json:"user_name"`
	UserUsername    string                               `json:"user_username"`
	UserPhone       string                               `json:"user_phone"`
	UserRemark      string                               `json:"user_remark"`
	UserAvatar      string                               `json:"user_avatar"`
	UserPush        automationMetadataMemberPushResponse `json:"user_push"`
	RoleName        string                               `json:"role_name"`
	RoleDescription string                               `json:"role_description"`
	RoleIsSystem    string                               `json:"role_is_system"`
}

type automationMetadataVersionResponse struct {
	Id               string `json:"id"`
	AdminId          string `json:"admin_id"`
	WorkspaceId      string `json:"workspace_id"`
	Name             string `json:"name"`
	StartDate        string `json:"start_date"`
	EndDate          string `json:"end_date"`
	RequirementCount string `json:"requirement_count"`
	CompletedCount   string `json:"completed_count"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type automationWorkspaceMetadataResponse struct {
	CustomFieldMembers []workspaceMetadataCustomFieldMemberResponse `json:"custom_field_members"`
	Requirement        automationMetadataGroupResponse              `json:"requirement"`
	Defect             automationMetadataGroupResponse              `json:"defect"`
	Settings           automationMetadataSettingsResponse           `json:"settings"`
	Workspace          automationMetadataWorkspaceResponse          `json:"workspace"`
	Members            []automationMetadataMemberResponse           `json:"members"`
	Versions           []automationMetadataVersionResponse          `json:"versions"`
	Permissions        []string                                     `json:"permissions"`
}

type automationMetadataResponse struct {
	Fields            []automationMetadataFieldResponse      `json:"fields"`
	RequirementFields []automationMetadataFieldResponse      `json:"requirement_fields"`
	Variables         []automationMetadataVariableResponse   `json:"variables"`
	CopyTargets       []automationMetadataCopyTargetResponse `json:"copy_targets"`
	WorkspaceMetadata automationWorkspaceMetadataResponse    `json:"workspace_metadata"`
}

func toAutomationMetadataResponse(model business.AutomationMetadataModel) automationMetadataResponse {
	return automationMetadataResponse{
		Fields: toAutomationMetadataFieldsResponse(model.Fields), RequirementFields: toAutomationMetadataFieldsResponse(model.RequirementFields),
		Variables: toAutomationMetadataVariablesResponse(model.Variables), CopyTargets: toAutomationMetadataCopyTargetsResponse(model.CopyTargets),
		WorkspaceMetadata: toAutomationWorkspaceMetadataResponse(model.WorkspaceMetadata),
	}
}

func toAutomationMetadataFieldsResponse(models []business.AutomationMetadataFieldModel) []automationMetadataFieldResponse {
	items := make([]automationMetadataFieldResponse, 0, len(models))
	for _, field := range models {
		options := make([]automationMetadataFieldOptionResponse, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, automationMetadataFieldOptionResponse{Key: option.Key, Name: option.Name, Color: option.Color})
		}
		if field.Options == nil {
			options = nil
		}
		items = append(items, automationMetadataFieldResponse{
			Key: field.Key, Label: field.Label, Type: field.Type, Editable: field.Editable, Deleted: field.Deleted, Options: options, MemberScope: field.MemberScope, RoleIds: field.RoleIds,
		})
	}
	return items
}

func toAutomationMetadataVariablesResponse(models []business.AutomationVariableModel) []automationMetadataVariableResponse {
	items := make([]automationMetadataVariableResponse, 0, len(models))
	for _, variable := range models {
		items = append(items, automationMetadataVariableResponse{Key: variable.Key, Group: variable.Group, Label: variable.Label})
	}
	return items
}

func toAutomationMetadataCopyTargetsResponse(models []business.AutomationCopyTargetModel) []automationMetadataCopyTargetResponse {
	items := make([]automationMetadataCopyTargetResponse, 0, len(models))
	for _, target := range models {
		items = append(items, automationMetadataCopyTargetResponse{Id: strconv.Itoa(target.Id), Name: target.Name})
	}
	return items
}

func toAutomationWorkspaceMetadataResponse(model business.WorkspaceMetadataModel) automationWorkspaceMetadataResponse {
	workspace := model.Workspace
	response := automationWorkspaceMetadataResponse{
		Requirement: toAutomationMetadataGroupResponse(model.Requirement), Defect: toAutomationMetadataGroupResponse(model.Defect),
		Settings: automationMetadataSettingsResponse{
			RequirementInitialStatusId:    model.Settings.RequirementInitialStatusId,
			DefectInitialStatusId:         model.Settings.DefectInitialStatusId,
			RequirementFinalStatusIds:     model.Settings.RequirementFinalStatusIds,
			RequirementCompletedStatusIds: model.Settings.RequirementCompletedStatusIds,
			RequirementClosedStatusIds:    model.Settings.RequirementClosedStatusIds,
			DefectFinalStatusIds:          model.Settings.DefectFinalStatusIds,
			DefectCompletedStatusIds:      model.Settings.DefectCompletedStatusIds,
			DefectClosedStatusIds:         model.Settings.DefectClosedStatusIds,
		},
		CustomFieldMembers: toWorkspaceMetadataCustomFieldMembersResponse(model.CustomFieldMembers),
		Members:            make([]automationMetadataMemberResponse, 0, len(model.Members)),
		Versions:           make([]automationMetadataVersionResponse, 0, len(model.Versions)), Permissions: model.Permissions,
	}
	if workspace != nil {
		response.Workspace = automationMetadataWorkspaceResponse{
			Id: strconv.Itoa(workspace.Id), AdminId: strconv.Itoa(workspace.AdminId), Name: workspace.Name, Slug: workspace.Slug,
			Description: workspace.Description, Color: workspace.Color, Icon: workspace.Icon, EndedAt: strconv.FormatInt(workspace.EndedAt, 10),
			ActiveVersionId: strconv.Itoa(workspace.ActiveVersionId), CreatedBy: strconv.Itoa(workspace.CreatedBy),
			CreatedAt: strconv.FormatInt(workspace.CreatedAt, 10), UpdatedAt: strconv.FormatInt(workspace.UpdatedAt, 10),
		}
	}
	for _, member := range model.Members {
		response.Members = append(response.Members, automationMetadataMemberResponse{
			Id: strconv.Itoa(member.Id), AdminId: strconv.Itoa(member.AdminId), WorkspaceId: strconv.Itoa(member.WorkspaceId),
			UserId: strconv.Itoa(member.UserId), RoleId: strconv.Itoa(member.RoleId), MemberType: member.MemberType,
			JoinedAt: strconv.FormatInt(member.JoinedAt, 10), CreatedAt: strconv.FormatInt(member.CreatedAt, 10), UpdatedAt: strconv.FormatInt(member.UpdatedAt, 10),
			UserName: member.UserName, UserUsername: member.UserUsername, UserPhone: member.UserPhone, UserRemark: member.UserRemark, UserAvatar: member.UserAvatar,
			UserPush: automationMetadataMemberPushResponse{
				DingtalkAtMobiles:        member.UserPush.DingtalkAtMobiles,
				DingtalkAtUserIds:        member.UserPush.DingtalkAtUserIds,
				FeishuUserId:             member.UserPush.FeishuUserId,
				WecomMentionedList:       member.UserPush.WecomMentionedList,
				WecomMentionedMobileList: member.UserPush.WecomMentionedMobileList,
			},
			RoleName: member.RoleName, RoleDescription: member.RoleDescription, RoleIsSystem: strconv.Itoa(member.RoleIsSystem),
		})
	}
	for _, version := range model.Versions {
		response.Versions = append(response.Versions, automationMetadataVersionResponse{
			Id: strconv.Itoa(version.Id), AdminId: strconv.Itoa(version.AdminId), WorkspaceId: strconv.Itoa(version.WorkspaceId), Name: version.Name,
			StartDate: strconv.FormatInt(version.StartDate, 10), EndDate: strconv.FormatInt(version.EndDate, 10),
			RequirementCount: strconv.Itoa(version.RequirementCount), CompletedCount: strconv.Itoa(version.CompletedCount),
			CreatedAt: strconv.FormatInt(version.CreatedAt, 10), UpdatedAt: strconv.FormatInt(version.UpdatedAt, 10),
		})
	}
	return response
}

func toAutomationMetadataGroupResponse(model business.WorkspaceMetadataGroupModel) automationMetadataGroupResponse {
	response := automationMetadataGroupResponse{
		Statuses:     make([]automationMetadataStatusResponse, 0, len(model.Statuses)),
		Priorities:   make([]automationMetadataPriorityResponse, 0, len(model.Priorities)),
		FieldConfigs: make([]automationMetadataFieldConfigResponse, 0, len(model.FieldConfigs)),
		CustomFields: make([]automationMetadataCustomFieldResponse, 0, len(model.CustomFields)),
	}
	for _, item := range model.Statuses {
		response.Statuses = append(response.Statuses, automationMetadataStatusResponse(item))
	}
	for _, item := range model.Priorities {
		response.Priorities = append(response.Priorities, automationMetadataPriorityResponse(item))
	}
	for _, item := range model.FieldConfigs {
		response.FieldConfigs = append(response.FieldConfigs, automationMetadataFieldConfigResponse(item))
	}
	for _, item := range model.CustomFields {
		options := make([]automationMetadataCustomFieldOptionResponse, 0, len(item.Options))
		for _, option := range item.Options {
			options = append(options, automationMetadataCustomFieldOptionResponse(option))
		}
		if item.Options == nil {
			options = nil
		}
		response.CustomFields = append(response.CustomFields, automationMetadataCustomFieldResponse{
			Id: item.Id, Key: item.Key, Name: item.Name, FieldType: item.FieldType, IsRequired: item.IsRequired,
			SortOrder: item.SortOrder, IsEnabled: item.IsEnabled, Options: options,
		})
	}
	return response
}
