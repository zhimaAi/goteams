// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// IterationStatusOptionModel is a status available to an iteration work-item filter.
type IterationStatusOptionModel struct {
	Id          int
	WorkspaceId int
	Type        string
	Name        string
	Color       string
}

// IterationPriorityOptionModel is a priority available to an iteration work-item filter.
type IterationPriorityOptionModel struct {
	Id          int
	WorkspaceId int
	Type        string
	Name        string
	Color       string
}

// IterationMemberOptionModel is a tenant member available to an iteration work-item filter.
type IterationMemberOptionModel struct {
	UserId   int
	Name     string
	Username string
	Avatar   string
}

// IterationFilterOptionsModel contains filter sources visible to the requester.
type IterationFilterOptionsModel struct {
	Workspaces     []VersionWorkspaceModel
	Statuses       []IterationStatusOptionModel
	Priorities     []IterationPriorityOptionModel
	Members        []IterationMemberOptionModel
	FinalStatusIds []int
}

type iterationWorkspaceAccess struct {
	Workspace       VersionWorkspaceModel
	ViewRequirement bool
	ViewDefect      bool
}

func (access iterationWorkspaceAccess) canView(itemType string) bool {
	if itemType == define.TypeKeyDefect {
		return access.ViewDefect
	}
	return access.ViewRequirement
}

// ListVersionWorkspaceOptions returns active workspaces where the user can create iterations.
func ListVersionWorkspaceOptions(adminID, userID int) ([]VersionWorkspaceOptionModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, name FROM workspaces
		WHERE admin_id = $1 AND ended_at = 0 ORDER BY name, id`, nil, adminID)
	if err != nil {
		logs.Error("list iteration workspace options failed, err:%s", err.Error())
		return nil, err
	}
	items := make([]VersionWorkspaceOptionModel, 0, len(rows))
	for _, row := range rows {
		workspaceId := cast.ToInt(row[`id`])
		if HasPermission(adminID, userID, cast.ToString(workspaceId), `version:create`) {
			items = append(items, VersionWorkspaceOptionModel{Id: workspaceId, Name: row[`name`]})
		}
	}
	return items, nil
}

func listIterationWorkspaceAccesses(adminID, userID, currentWorkspaceID, versionID int) ([]iterationWorkspaceAccess, error) {
	version, err := GetVersionByWorkspace(adminID, currentWorkspaceID, versionID)
	if err != nil {
		return nil, err
	}
	accesses := make([]iterationWorkspaceAccess, 0, len(version.Workspaces))
	for _, workspace := range version.Workspaces {
		access := iterationWorkspaceAccess{
			Workspace:       workspace,
			ViewRequirement: HasPermission(adminID, userID, cast.ToString(workspace.Id), `requirement:view`),
			ViewDefect:      HasPermission(adminID, userID, cast.ToString(workspace.Id), `defect:view`),
		}
		if access.ViewRequirement || access.ViewDefect {
			accesses = append(accesses, access)
		}
	}
	return accesses, nil
}

// GetIterationFilterOptions returns filter sources from workspaces visible to the requester.
func GetIterationFilterOptions(adminID, userID, currentWorkspaceID, versionID int) (IterationFilterOptionsModel, error) {
	result := IterationFilterOptionsModel{
		Workspaces: []VersionWorkspaceModel{}, Statuses: []IterationStatusOptionModel{},
		Priorities: []IterationPriorityOptionModel{}, Members: []IterationMemberOptionModel{}, FinalStatusIds: []int{},
	}
	accesses, err := listIterationWorkspaceAccesses(adminID, userID, currentWorkspaceID, versionID)
	if err != nil {
		return result, err
	}
	if len(accesses) == 0 {
		return result, nil
	}
	workspaceIds := make([]int, 0, len(accesses))
	accessByWorkspace := make(map[int]iterationWorkspaceAccess, len(accesses))
	for _, access := range accesses {
		result.Workspaces = append(result.Workspaces, access.Workspace)
		workspaceIds = append(workspaceIds, access.Workspace.Id)
		accessByWorkspace[access.Workspace.Id] = access
		settings, settingsErr := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: adminID, WsId: cast.ToString(access.Workspace.Id)})
		if settingsErr != nil {
			return result, settingsErr
		}
		if access.ViewRequirement {
			result.FinalStatusIds = append(result.FinalStatusIds, settings.RequirementFinalStatusIds...)
		}
		if access.ViewDefect {
			result.FinalStatusIds = append(result.FinalStatusIds, settings.DefectFinalStatusIds...)
		}
	}
	statusRows, err := msql.RawValues(define.Postgres, `SELECT id, workspace_id, type_key, name, color
		FROM workspace_statuses WHERE admin_id = $1 AND workspace_id = ANY($2)
		AND type_key IN ($3, $4) ORDER BY workspace_id, type_key, sort_order, id`, nil,
		adminID, pq.Array(workspaceIds), define.TypeKeyRequirement, define.TypeKeyDefect)
	if err != nil {
		logs.Error("list iteration status options failed, err:%s", err.Error())
		return result, err
	}
	for _, row := range statusRows {
		workspaceId := cast.ToInt(row[`workspace_id`])
		if accessByWorkspace[workspaceId].canView(row[`type_key`]) {
			result.Statuses = append(result.Statuses, IterationStatusOptionModel{
				Id: cast.ToInt(row[`id`]), WorkspaceId: workspaceId, Type: row[`type_key`], Name: row[`name`], Color: row[`color`],
			})
		}
	}
	priorityRows, err := msql.RawValues(define.Postgres, `SELECT id, workspace_id, type_key, name, color
		FROM workspace_priorities WHERE admin_id = $1 AND workspace_id = ANY($2)
		AND type_key IN ($3, $4) ORDER BY workspace_id, type_key, sort_order, id`, nil,
		adminID, pq.Array(workspaceIds), define.TypeKeyRequirement, define.TypeKeyDefect)
	if err != nil {
		logs.Error("list iteration priority options failed, err:%s", err.Error())
		return result, err
	}
	for _, row := range priorityRows {
		workspaceId := cast.ToInt(row[`workspace_id`])
		if accessByWorkspace[workspaceId].canView(row[`type_key`]) {
			result.Priorities = append(result.Priorities, IterationPriorityOptionModel{
				Id: cast.ToInt(row[`id`]), WorkspaceId: workspaceId, Type: row[`type_key`], Name: row[`name`], Color: row[`color`],
			})
		}
	}
	memberRows, err := msql.RawValues(define.Postgres, `SELECT DISTINCT u.id, u.name, u.username, u.avatar
		FROM workspace_members wm JOIN users u ON u.admin_id = wm.admin_id AND u.id = wm.user_id
		WHERE wm.admin_id = $1 AND wm.workspace_id = ANY($2) AND COALESCE(u.status, 1) = 1
		ORDER BY u.name, u.id`, nil, adminID, pq.Array(workspaceIds))
	if err != nil {
		logs.Error("list iteration member options failed, err:%s", err.Error())
		return result, err
	}
	for _, row := range memberRows {
		result.Members = append(result.Members, IterationMemberOptionModel{
			UserId: cast.ToInt(row[`id`]), Name: row[`name`], Username: row[`username`], Avatar: row[`avatar`],
		})
	}
	return result, nil
}
