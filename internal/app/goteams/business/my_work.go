package business

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/zhimaAi/go_tools/msql"
)

// CrossWorkspaceWorkItemModel 是我的工作与最近访问共同使用的工作项投影。
type CrossWorkspaceWorkItemModel struct {
	Id                 int
	AdminId            int
	WorkspaceId        int
	ParentId           *int
	Title              string
	Description        string
	Type               string
	Priority           *int
	StatusId           *int
	VersionId          *int
	PlannedStartDate   int64
	PlannedEndDate     int64
	WorkHours          *float64
	CompletedWorkHours float64
	CommentCount       *int
	CreatedBy          *int
	CreatedAt          int64
	UpdatedAt          int64
	WorkspaceName      string
	WorkspaceColor     string
	StatusName         string
	StatusColor        string
	StatusSortOrder    *int
	PriorityName       string
	PriorityColor      string
	PrioritySortOrder  *int
	AssigneeIds        []int
	AssigneeNames      []string
	AssigneeAvatars    []string
	AssigneeUsernames  []string
	ParentTitle        string
	IsDone             bool
	IsCompleted        bool
	CanEdit            bool
	VisitedAt          *int64
}

// MyWorkWorkspaceModel 是允许当前用户创建工作项的工作空间。
type MyWorkWorkspaceModel struct {
	Id          int
	Name, Color string
}

// MyWorkModel 是我的工作列表、可创建工作空间及最近访问数量。
type MyWorkModel struct {
	Items               []CrossWorkspaceWorkItemModel
	CreatableWorkspaces []MyWorkWorkspaceModel
	RecentVisitCount    int
}

func toCrossWorkspaceWorkItems(rows []msql.Params) ([]CrossWorkspaceWorkItemModel, error) {
	items := make([]CrossWorkspaceWorkItemModel, 0, len(rows))
	for _, row := range rows {
		var item CrossWorkspaceWorkItemModel
		var err error
		item.Id, err = strconv.Atoi(row["id"])
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace id: %w", err)
		}
		item.AdminId, err = strconv.Atoi(row["admin_id"])
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace admin_id: %w", err)
		}
		item.WorkspaceId, err = strconv.Atoi(row["workspace_id"])
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace workspace_id: %w", err)
		}
		if row["parent_id"] != "" {
			value, err := strconv.Atoi(row["parent_id"])
			if err != nil {
				return nil, fmt.Errorf("parse cross-workspace parent_id: %w", err)
			}
			item.ParentId = &value
		}
		item.Title = row["title"]
		item.Description = row["description"]
		item.Type = row["type"]
		if row["priority"] != "" {
			var v int
			v, err = strconv.Atoi(row["priority"])
			item.Priority = &v
		}
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace priority: %w", err)
		}
		if row["status_id"] != "" {
			value, err := strconv.Atoi(row["status_id"])
			if err != nil {
				return nil, fmt.Errorf("parse cross-workspace status_id: %w", err)
			}
			item.StatusId = &value
		}
		if row["version_id"] != "" {
			v, err := strconv.Atoi(row["version_id"])
			if err != nil {
				return nil, fmt.Errorf("parse cross-workspace version_id: %w", err)
			}
			item.VersionId = &v
		}
		item.PlannedStartDate, err = strconv.ParseInt(row["planned_start_date"], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace planned_start_date: %w", err)
		}
		item.PlannedEndDate, err = strconv.ParseInt(row["planned_end_date"], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace planned_end_date: %w", err)
		}
		if row["work_hours"] != "" {
			value, err := strconv.ParseFloat(row["work_hours"], 64)
			if err != nil {
				return nil, fmt.Errorf("parse cross-workspace work_hours: %w", err)
			}
			item.WorkHours = &value
		}
		item.CompletedWorkHours, err = parseRequirementFloat64(row, "completed_work_hours")
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace completed_work_hours: %w", err)
		}
		if row["comment_count"] != "" {
			value, err := strconv.Atoi(row["comment_count"])
			if err != nil {
				return nil, fmt.Errorf("parse cross-workspace comment_count: %w", err)
			}
			item.CommentCount = &value
		}
		if row["created_by"] != "" {
			value, err := strconv.Atoi(row["created_by"])
			if err != nil {
				return nil, fmt.Errorf("parse cross-workspace created_by: %w", err)
			}
			item.CreatedBy = &value
		}
		item.CreatedAt, err = strconv.ParseInt(row["created_at"], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace created_at: %w", err)
		}
		item.UpdatedAt, err = strconv.ParseInt(row["updated_at"], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace updated_at: %w", err)
		}
		item.WorkspaceName = row["workspace_name"]
		item.WorkspaceColor = row["workspace_color"]
		item.StatusName = row["status_name"]
		item.StatusColor = row["status_color"]
		if row["status_sort_order"] != "" {
			var v int
			v, err = strconv.Atoi(row["status_sort_order"])
			item.StatusSortOrder = &v
		}
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace status_sort_order: %w", err)
		}
		item.PriorityName = row["priority_name"]
		item.PriorityColor = row["priority_color"]
		if row["priority_sort_order"] != "" {
			var v int
			v, err = strconv.Atoi(row["priority_sort_order"])
			item.PrioritySortOrder = &v
		}
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace priority_sort_order: %w", err)
		}
		err = json.Unmarshal([]byte(row["assignee_ids"]), &item.AssigneeIds)
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace assignee_ids: %w", err)
		}
		err = json.Unmarshal([]byte(row["assignee_names"]), &item.AssigneeNames)
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace assignee_names: %w", err)
		}
		err = json.Unmarshal([]byte(row["assignee_avatars"]), &item.AssigneeAvatars)
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace assignee_avatars: %w", err)
		}
		err = json.Unmarshal([]byte(row["assignee_usernames"]), &item.AssigneeUsernames)
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace assignee_usernames: %w", err)
		}
		item.ParentTitle = row["parent_title"]
		item.IsDone, err = strconv.ParseBool(row["is_done"])
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace is_done: %w", err)
		}
		item.IsCompleted, err = strconv.ParseBool(row["is_completed"])
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace is_completed: %w", err)
		}
		item.CanEdit, err = strconv.ParseBool(row["can_edit"])
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace can_edit: %w", err)
		}
		if v, exists := row["visited_at"]; exists {
			var value int64
			value, err = strconv.ParseInt(v, 10, 64)
			item.VisitedAt = &value
		}
		if err != nil {
			return nil, fmt.Errorf("parse cross-workspace visited_at: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}
