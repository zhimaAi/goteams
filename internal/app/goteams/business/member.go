// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"fmt"
	"strconv"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	ErrWorkspaceMemberAlreadyExists    = errors.New("workspace member already exists")
	ErrWorkspaceRoleNotFound           = errors.New("workspace role not found")
	ErrWorkspaceMemberCandidateInvalid = errors.New("workspace member candidate invalid")
	ErrCopySourceWorkspaceInvalid      = errors.New("copy source workspace invalid")
	ErrCopySourceWorkspaceDenied       = errors.New("copy source workspace denied")
)

// WorkspaceMemberPushModel describes the mention parameters a member owns on every robot.
type WorkspaceMemberPushModel struct {
	DingtalkAtMobiles        string
	DingtalkAtUserIds        string
	FeishuUserId             string
	WecomMentionedList       string
	WecomMentionedMobileList string
}

// WorkspaceMemberModel describes a workspace member and its user and role data.
type WorkspaceMemberModel struct {
	Id               int
	AdminId          int
	WorkspaceId      int
	UserId           int
	RoleId           int
	EnterpriseRoleId int
	MemberType       string
	JoinedAt         int64
	CreatedAt        int64
	UpdatedAt        int64
	UserName         string
	UserUsername     string
	UserPhone        string
	UserRemark       string
	UserAvatar       string
	UserStatus       int
	UserPush         WorkspaceMemberPushModel
	RoleName         string
	RoleDescription  string
	RoleIsSystem     int
	// IsOwner marks the workspace's default project admin: cannot be removed, transferable.
	IsOwner bool
	// Removed marks a soft-deleted tenant member kept only so historical work items still render
	// their name and avatar; pickers must filter these out (they can no longer be assigned).
	Removed bool
}

// ListWorkspaceMembersModel describes the workspace member list use case.
type ListWorkspaceMembersModel struct {
	AdminId     int
	WorkspaceId string
	Items       []WorkspaceMemberModel
}

// MemberCandidateModel describes an enterprise member offered by the project member picker,
// carrying the department and enterprise role links the picker trees are grouped by.
type MemberCandidateModel struct {
	Id            int
	Username      string
	Name          string
	Avatar        string
	RoleId        int
	DepartmentIds []int
	Joined        bool
}

// ListMemberCandidatesModel describes the project member candidate use case: the organization
// tree source data plus every enterprise member with its join state in the target workspace.
type ListMemberCandidatesModel struct {
	AdminId         int
	WorkspaceId     string
	EnterpriseName  string
	Departments     []DepartmentModel
	EnterpriseRoles []EnterpriseRoleBriefModel
	Items           []MemberCandidateModel
}

// ListMembersByWorkspace returns the workspace member list (admin_id isolated, joining user info).
func ListMembersByWorkspace(model ListWorkspaceMembersModel) (ListWorkspaceMembersModel, error) {
	m := msql.Model(define.TableWorkspaceMembers, define.Postgres).
		Alias("wm").
		Field(`wm.id, wm.admin_id, wm.workspace_id, wm.user_id, wm.role_id, wm.member_type, wm.joined_at, wm.created_at, wm.updated_at`).
		Field("u.role_id as enterprise_role_id").
		Field("u.name as user_name").
		Field("u.username as user_username").
		Field("u.tel_number as user_phone").
		Field("u.remark as user_remark").
		Field("u.avatar as user_avatar").
		Field("COALESCE(u.status, 1) as user_status").
		Field("u.push_dingtalk_at_mobiles as user_dingtalk_at_mobiles").
		Field("u.push_dingtalk_at_user_ids as user_dingtalk_at_user_ids").
		Field("u.push_feishu_user_id as user_feishu_user_id").
		Field("u.push_wecom_mentioned_list as user_wecom_mentioned_list").
		Field("u.push_wecom_mentioned_mobile_list as user_wecom_mentioned_mobile_list").
		Field("r.name as role_name").
		Field("r.description as role_description").
		Field("r.is_system as role_is_system")
	rows, err := m.Where2([][]string{
		{`wm.workspace_id`, model.WorkspaceId},
		{`wm.admin_id`, cast.ToString(model.AdminId)},
	}).Join("users u", "wm.user_id = u.id AND wm.admin_id = u.admin_id", "left").
		Join("roles r", "wm.role_id = r.id AND wm.admin_id = r.admin_id", "left").
		Order(`wm.id asc`).
		Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return ListWorkspaceMembersModel{}, err
	}
	model.Items = make([]WorkspaceMemberModel, 0, len(rows))
	ownerUserId, err := GetWorkspaceOwnerId(model.AdminId, cast.ToInt(model.WorkspaceId))
	if err != nil {
		return ListWorkspaceMembersModel{}, err
	}
	for _, row := range rows {
		member, err := toWorkspaceMemberModel(row)
		if err != nil {
			logs.Error("convert workspace member row error: %s", err.Error())
			return ListWorkspaceMembersModel{}, err
		}
		member.IsOwner = ownerUserId > 0 && member.UserId == ownerUserId
		model.Items = append(model.Items, member)
	}
	return model, nil
}

// ListWorkspaceFormerAssignees returns tenant users that the project's work items still reference
// as assignees but that no longer appear in its member list. Removing a project member only drops
// the workspace_members row (the users row stays alive with deleted_at = 0), so
// ListRemovedTenantMembers never reports them and the planning table, which resolves assignee
// names from the member list, would render them as an empty cell. Returned with Removed set, so
// pickers keep filtering them out and they cannot be assigned again.
// knownUserIds carries the user ids already present in the metadata member list.
// ponytail: scans the project's own work items on every metadata read; add a covering index or a
// cache if a project ever grows past a few thousand items.
func ListWorkspaceFormerAssignees(adminId int, wsId string, knownUserIds map[int]bool) ([]WorkspaceMemberModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT DISTINCT unnest(assignee_ids) AS user_id
		FROM `+define.TableRequirements+`
		WHERE admin_id = $1 AND workspace_id = $2 AND deleted_at = 0`,
		nil, adminId, cast.ToInt(wsId))
	if err != nil {
		logs.Error("list workspace assignee ids error: %s", err.Error())
		return nil, err
	}

	missingIds := make([]int, 0, len(rows))
	for _, row := range rows {
		userId := cast.ToInt(row[`user_id`])
		if userId > 0 && !knownUserIds[userId] {
			missingIds = append(missingIds, userId)
		}
	}
	if len(missingIds) == 0 {
		return nil, nil
	}

	userRows, err := msql.RawValues(define.Postgres, `SELECT id, name, avatar FROM `+define.TableUsers+`
		WHERE admin_id = $1 AND id = ANY($2) ORDER BY id`, nil, adminId, pq.Array(missingIds))
	if err != nil {
		logs.Error("list workspace former assignees error: %s", err.Error())
		return nil, err
	}
	items := make([]WorkspaceMemberModel, 0, len(userRows))
	for _, row := range userRows {
		items = append(items, WorkspaceMemberModel{
			AdminId:    adminId,
			UserId:     cast.ToInt(row[`id`]),
			UserName:   row[`name`],
			UserAvatar: row[`avatar`],
			Removed:    true,
		})
	}
	return items, nil
}

// ListMemberCandidates returns everything the project member picker needs in one round trip:
// the enterprise name (organization tree root), the department list, the enterprise role list and
// every tenant member flagged with whether it already joined the workspace (admin_id isolated).
func ListMemberCandidates(model ListMemberCandidatesModel) (ListMemberCandidatesModel, error) {
	enterpriseName, err := GetEnterpriseName(model.AdminId)
	if err != nil {
		return ListMemberCandidatesModel{}, err
	}
	departments, err := ListDepartments(model.AdminId)
	if err != nil {
		return ListMemberCandidatesModel{}, err
	}
	roles, err := ListEnterpriseRoleBriefs(model.AdminId)
	if err != nil {
		return ListMemberCandidatesModel{}, err
	}
	members, err := ListMembersByWorkspace(ListWorkspaceMembersModel{AdminId: model.AdminId, WorkspaceId: model.WorkspaceId})
	if err != nil {
		return ListMemberCandidatesModel{}, err
	}
	joined := make(map[int]bool, len(members.Items))
	for _, member := range members.Items {
		joined[member.UserId] = true
	}
	all, err := ListEnterpriseMembers(ListEnterpriseMembersModel{AdminId: model.AdminId})
	if err != nil {
		return ListMemberCandidatesModel{}, err
	}

	model.EnterpriseName = enterpriseName
	model.Departments = departments
	model.EnterpriseRoles = roles
	model.Items = make([]MemberCandidateModel, 0, len(all.Items))
	for _, member := range all.Items {
		// Disabled members cannot be added to a workspace: joining is a new assignment,
		// and a stopped account should not be pulled into new projects.
		if member.Status == define.MemberStatusDisabled {
			continue
		}
		model.Items = append(model.Items, MemberCandidateModel{
			Id:            member.Id,
			Username:      member.Username,
			Name:          member.Name,
			Avatar:        member.Avatar,
			RoleId:        member.RoleId,
			DepartmentIds: member.DepartmentIds,
			Joined:        joined[member.Id],
		})
	}
	return model, nil
}

func toWorkspaceMemberModel(row msql.Params) (WorkspaceMemberModel, error) {
	parseInt := func(key string) (int, error) {
		value, err := strconv.Atoi(row[key])
		if err != nil {
			return 0, fmt.Errorf("parse workspace member %s: %w", key, err)
		}
		return value, nil
	}
	parseInt64 := func(key string) (int64, error) {
		value, err := strconv.ParseInt(row[key], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse workspace member %s: %w", key, err)
		}
		return value, nil
	}
	id, err := parseInt(`id`)
	if err != nil {
		return WorkspaceMemberModel{}, err
	}
	adminId, err := parseInt(`admin_id`)
	if err != nil {
		return WorkspaceMemberModel{}, err
	}
	workspaceId, err := parseInt(`workspace_id`)
	if err != nil {
		return WorkspaceMemberModel{}, err
	}
	userId, err := parseInt(`user_id`)
	if err != nil {
		return WorkspaceMemberModel{}, err
	}
	roleId, err := parseInt(`role_id`)
	if err != nil {
		return WorkspaceMemberModel{}, err
	}
	joinedAt, err := parseInt64(`joined_at`)
	if err != nil {
		return WorkspaceMemberModel{}, err
	}
	createdAt, err := parseInt64(`created_at`)
	if err != nil {
		return WorkspaceMemberModel{}, err
	}
	updatedAt, err := parseInt64(`updated_at`)
	if err != nil {
		return WorkspaceMemberModel{}, err
	}
	roleIsSystem, err := parseInt(`role_is_system`)
	if err != nil {
		return WorkspaceMemberModel{}, err
	}
	enterpriseRoleId := 0
	if row[`enterprise_role_id`] != `` {
		enterpriseRoleId, err = parseInt(`enterprise_role_id`)
		if err != nil {
			return WorkspaceMemberModel{}, err
		}
	}
	return WorkspaceMemberModel{
		Id: id, AdminId: adminId, WorkspaceId: workspaceId, UserId: userId, RoleId: roleId,
		EnterpriseRoleId: enterpriseRoleId,
		MemberType:       row[`member_type`], JoinedAt: joinedAt, CreatedAt: createdAt, UpdatedAt: updatedAt,
		UserName: row[`user_name`], UserUsername: row[`user_username`], UserPhone: row[`user_phone`], UserRemark: row[`user_remark`], UserAvatar: row[`user_avatar`],
		UserStatus: cast.ToInt(row[`user_status`]),
		UserPush: WorkspaceMemberPushModel{
			DingtalkAtMobiles:        row[`user_dingtalk_at_mobiles`],
			DingtalkAtUserIds:        row[`user_dingtalk_at_user_ids`],
			FeishuUserId:             row[`user_feishu_user_id`],
			WecomMentionedList:       row[`user_wecom_mentioned_list`],
			WecomMentionedMobileList: row[`user_wecom_mentioned_mobile_list`],
		},
		RoleName: row[`role_name`], RoleDescription: row[`role_description`], RoleIsSystem: roleIsSystem,
	}, nil
}

// AddMemberToWorkspace adds a member and their notification in one transaction.
func AddMemberToWorkspace(adminId int, wsId, userId, role string, operatorID int) (bool, error) {
	if !projectRoleExists(adminId, wsId, cast.ToInt(role)) {
		return false, ErrWorkspaceRoleNotFound
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	const lockQuery = `SELECT id FROM ` + define.TableWorkspaces + ` WHERE admin_id = $1 AND id = $2 FOR UPDATE`
	if _, err = msql.RawValues(define.Postgres, lockQuery, tx, adminId, cast.ToInt(wsId)); err != nil {
		logs.Error("sql:%s, err:%s", lockQuery, err.Error())
		return false, err
	}
	const userQuery = `SELECT id FROM ` + define.TableUsers + ` WHERE admin_id = $1 AND id = $2 AND status = 1 AND deleted_at = 0`
	users, err := msql.RawValues(define.Postgres, userQuery, tx, adminId, cast.ToInt(userId))
	if err != nil {
		logs.Error("sql:%s, err:%s", userQuery, err.Error())
		return false, err
	}
	if len(users) == 0 {
		return false, ErrWorkspaceMemberCandidateInvalid
	}
	now := tool.Time2Int()
	const insertQuery = `INSERT INTO ` + define.TableWorkspaceMembers + `
		(admin_id, workspace_id, user_id, role_id, joined_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5, $5) ON CONFLICT (workspace_id, user_id) DO NOTHING`
	result, err := msql.RawExec(define.Postgres, insertQuery, tx,
		adminId, cast.ToInt(wsId), cast.ToInt(userId), cast.ToInt(role), now)
	if err != nil {
		logs.Error("sql:%s, err:%s", insertQuery, err.Error())
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	const updateQuery = `UPDATE ` + define.TableWorkspaceAccessRequests + ` SET status = 'joined', updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND user_id = $4 AND status = 'pending'`
	if _, err = msql.RawExec(define.Postgres, updateQuery, tx,
		now, adminId, cast.ToInt(wsId), cast.ToInt(userId)); err != nil {
		logs.Error("sql:%s, err:%s", updateQuery, err.Error())
		return false, err
	}
	if err = CreateWorkspaceNotificationTx(tx, notificationEvent{
		AdminID: adminId, RecipientID: cast.ToInt(userId), WorkspaceID: cast.ToInt(wsId),
		EventType: define.NotificationEventWorkspaceMemberAdded, ActorID: operatorID,
	}); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveWorkspaceMemberModel describes the remove-project-member use case.
// TransferItemsToUserId is optional: when set, the member's unfinished work items in this project
// move to that user, replacing only them inside assignee_ids.
type RemoveWorkspaceMemberModel struct {
	AdminId               int
	WorkspaceId           int
	OperatorUserId        int
	UserId                int
	TransferItemsToUserId int
}

// WorkspaceMemberRemovalPreviewModel reports how many unfinished work items the member holds in
// this project, so the dialog only offers a hand-over when there is something to hand over.
type WorkspaceMemberRemovalPreviewModel struct {
	AdminId         int
	WorkspaceId     int
	UserId          int
	UnfinishedCount int
}

// PreviewWorkspaceMemberRemoval collects what the project member removal dialog needs.
func PreviewWorkspaceMemberRemoval(model WorkspaceMemberRemovalPreviewModel) (WorkspaceMemberRemovalPreviewModel, error) {
	count, err := CountUnfinishedAssignedItems(model.AdminId, model.WorkspaceId, model.UserId)
	if err != nil {
		return WorkspaceMemberRemovalPreviewModel{}, err
	}
	model.UnfinishedCount = count
	return model, nil
}

// RemoveMemberFromWorkspace removes a workspace member (admin_id isolated).
// The default project admin is protected: their role has to be transferred first.
func RemoveMemberFromWorkspace(model RemoveWorkspaceMemberModel) error {
	ownerUserId, err := GetWorkspaceOwnerId(model.AdminId, model.WorkspaceId)
	if err != nil {
		return err
	}
	if ownerUserId > 0 && ownerUserId == model.UserId {
		return ErrProjectOwnerProtected
	}
	if model.TransferItemsToUserId > 0 {
		if err = validateTransferTarget(model.AdminId, model.UserId, model.TransferItemsToUserId); err != nil {
			return err
		}
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin remove workspace member error: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if model.TransferItemsToUserId > 0 {
		if _, err = transferUnfinishedAssignedItems(tx, model.AdminId, model.WorkspaceId,
			model.UserId, model.TransferItemsToUserId, model.OperatorUserId); err != nil {
			return err
		}
		if err = ensureWorkspaceMembershipIfMissing(tx, model.AdminId, model.WorkspaceId, model.TransferItemsToUserId); err != nil {
			return err
		}
	}

	if err = deleteDashboardMemberRelationsTx(tx, model.AdminId, model.WorkspaceId, model.UserId); err != nil {
		return err
	}

	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableWorkspaceMembers+`
		WHERE admin_id = $1 AND workspace_id = $2 AND user_id = $3`,
		tx, model.AdminId, model.WorkspaceId, model.UserId); err != nil {
		logs.Error("delete workspace member error: %s", err.Error())
		return err
	}

	if err = tx.Commit(); err != nil {
		logs.Error("commit remove workspace member error: %s", err.Error())
		return err
	}
	return nil
}

// GetMemberRole queries a user's role in a workspace (admin_id isolated).
func GetMemberRole(adminId int, userId int, wsId string) string {
	if wsId == `` || userId == 0 {
		return ``
	}
	m := msql.Model(define.TableWorkspaceMembers, define.Postgres)
	role, err := m.Where2([][]string{
		{`user_id`, cast.ToString(userId)},
		{`workspace_id`, wsId},
		{`admin_id`, cast.ToString(adminId)},
	}).Value(`role_id`)
	if err != nil || len(role) == 0 {
		return ``
	}
	return role
}

// IsWorkspaceMember checks whether a user is a member of the workspace (admin_id isolated).
func IsWorkspaceMember(adminId int, userId int, wsId string) bool {
	return len(GetMemberRole(adminId, userId, wsId)) > 0
}

// UpdateMemberRole updates a member's role in a workspace (admin_id isolated).
func UpdateMemberRole(adminId int, wsId string, userId int, newRole string) error {
	if !projectRoleExists(adminId, wsId, cast.ToInt(newRole)) {
		return ErrWorkspaceRoleNotFound
	}

	m := msql.Model(define.TableWorkspaceMembers, define.Postgres)
	_, err := m.Where2([][]string{
		{`workspace_id`, wsId},
		{`user_id`, cast.ToString(userId)},
		{`admin_id`, cast.ToString(adminId)},
	}).Update(msql.Datas{
		`role_id`:    newRole,
		`updated_at`: tool.Time2Int(),
	})
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
	}
	return err
}

// AddMembersToWorkspace adds enterprise members to a workspace in batch (deduplication + role validation).
// Return the number of members successfully joined. Already-existing members are skipped.
func AddMembersToWorkspace(adminId int, wsId string, userIds []int, roleId, operatorID int) (int, error) {
	if !projectRoleExists(adminId, wsId, roleId) {
		return 0, ErrWorkspaceRoleNotFound
	}

	added := 0
	for _, userId := range userIds {
		if userId == 0 {
			continue
		}
		joined, err := AddMemberToWorkspace(adminId, wsId, cast.ToString(userId), cast.ToString(roleId), operatorID)
		if err != nil {
			return added, err
		}
		if joined {
			added++
		}
	}
	return added, nil
}

func projectRoleExists(adminId int, wsId string, roleId int) bool {
	if roleId == 0 {
		return false
	}
	m := msql.Model(define.TableRoles, define.Postgres)
	role, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`id`, cast.ToString(roleId)},
		{`scope`, permission.ScopeProject},
	}).Find()
	if err != nil {
		logs.Error("check project role exists error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return false
	}
	return len(role) > 0
}

// CopyProjectMembersModel describes the copy-roles-and-members-from-another-project use case.
type CopyProjectMembersModel struct {
	AdminId           int
	UserId            int
	WorkspaceId       string
	SourceWorkspaceId string
	RoleIds           []int
	CreatedRoles      int
	AddedMembers      int
}

// CopyProjectMembers copies the selected source project roles and their members into the target
// project. A target role with the same name is reused as-is (its permissions are never rewritten);
// a missing role is created with the source permissions. The partial data view scope is not
// portable across projects because its member set differs, so it degrades to "self".
// Members already in the target project are skipped.
func CopyProjectMembers(model CopyProjectMembersModel) (CopyProjectMembersModel, error) {
	if model.SourceWorkspaceId == `` || model.SourceWorkspaceId == model.WorkspaceId || len(model.RoleIds) == 0 {
		return CopyProjectMembersModel{}, ErrCopySourceWorkspaceInvalid
	}
	source, err := GetWorkspaceById(model.AdminId, model.SourceWorkspaceId)
	if err != nil {
		logs.Error("copy project members: load source workspace error: err=%s", err.Error())
		return CopyProjectMembersModel{}, err
	}
	if len(source) == 0 {
		return CopyProjectMembersModel{}, ErrCopySourceWorkspaceInvalid
	}
	// Only the tenant super admin or a member of the source project may read its roles and members.
	if !IsTenantOwner(model.AdminId, model.UserId) && !IsWorkspaceMember(model.AdminId, model.UserId, model.SourceWorkspaceId) {
		return CopyProjectMembersModel{}, ErrCopySourceWorkspaceDenied
	}

	sourceRoles, err := ListProjectRoles(model.AdminId, model.SourceWorkspaceId)
	if err != nil {
		return CopyProjectMembersModel{}, err
	}
	sourceRoleById := make(map[int]ProjectRoleInfo, len(sourceRoles))
	for _, role := range sourceRoles {
		sourceRoleById[role.Id] = role
	}
	targetRoles, err := ListProjectRoles(model.AdminId, model.WorkspaceId)
	if err != nil {
		return CopyProjectMembersModel{}, err
	}
	targetRoleIdByName := make(map[string]int, len(targetRoles))
	for _, role := range targetRoles {
		targetRoleIdByName[role.Name] = role.Id
	}

	sourceMembers, err := ListMembersByWorkspace(ListWorkspaceMembersModel{AdminId: model.AdminId, WorkspaceId: model.SourceWorkspaceId})
	if err != nil {
		return CopyProjectMembersModel{}, err
	}
	sourceUserIdsByRole := make(map[int][]int, len(sourceRoles))
	for _, member := range sourceMembers.Items {
		// Same rule as the member picker: a stopped account is never pulled into another project.
		if member.UserStatus == define.MemberStatusDisabled {
			continue
		}
		sourceUserIdsByRole[member.RoleId] = append(sourceUserIdsByRole[member.RoleId], member.UserId)
	}

	for _, roleId := range model.RoleIds {
		sourceRole, ok := sourceRoleById[roleId]
		if !ok {
			return CopyProjectMembersModel{}, ErrProjectRoleNotFound
		}
		targetRoleId, ok := targetRoleIdByName[sourceRole.Name]
		if !ok {
			targetRoleId, err = CreateProjectRole(model.AdminId, model.WorkspaceId, sourceRole.Name, sourceRole.Description, sourceRole.Permissions)
			if err != nil {
				return CopyProjectMembersModel{}, err
			}
			targetRoleIdByName[sourceRole.Name] = targetRoleId
			model.CreatedRoles++
			// CreateProjectRole defaults to "self"; only "all" needs an extra write.
			if sourceRole.DataViewScope == DataViewScopeAll {
				if err = UpdateProjectRoleDataViewScope(model.AdminId, model.WorkspaceId, targetRoleId, DataViewScopeAll, nil); err != nil {
					return CopyProjectMembersModel{}, err
				}
			}
		}
		added, err := AddMembersToWorkspace(model.AdminId, model.WorkspaceId, sourceUserIdsByRole[roleId], targetRoleId, model.UserId)
		if err != nil {
			return CopyProjectMembersModel{}, err
		}
		model.AddedMembers += added
	}
	return model, nil
}
