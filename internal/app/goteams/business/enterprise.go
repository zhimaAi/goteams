// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ======================== Enterprise Member Management ========================

var (
	ErrEnterpriseAdminProtected       = errors.New("enterprise admin cannot be modified")
	ErrEnterpriseMemberNotFound       = errors.New("enterprise member not found")
	ErrEnterpriseRoleNotFound         = errors.New("enterprise role not found")
	ErrEnterpriseRoleSystem           = errors.New("system enterprise role cannot be modified")
	ErrEnterpriseRoleInUse            = errors.New("enterprise role is in use")
	ErrEnterpriseRoleNameExists       = errors.New("enterprise role name already exists")
	ErrEnterpriseTeamWorkScopeInvalid = errors.New("invalid enterprise team work scope")
	ErrUserUsernameExists             = errors.New("user username exists")
	ErrMemberSelfStatusProtected      = errors.New("cannot change own account status")
	ErrMemberStatusInvalid            = errors.New("invalid member status")
)

// TeamWorkViewScopeAll Indicating team work where all members can be viewed
// TeamWorkViewScopePartial It indicates that only the team work of specified members can be viewed.
// TeamWorkViewScopeSelf It indicates that only your own team work can be viewed.
const (
	TeamWorkViewScopeAll     = "all"
	TeamWorkViewScopePartial = "partial"
	TeamWorkViewScopeSelf    = "self"
)

// Project role related errors (admin_id + workspace_id isolated).
var (
	ErrProjectRoleNameExists           = errors.New("project role name already exists")
	ErrProjectRoleNotFound             = errors.New("project role not found")
	ErrProjectRoleSystem               = errors.New("system project role cannot be modified")
	ErrProjectRoleInUse                = errors.New("project role is in use")
	ErrProjectRoleDataViewScopeInvalid = errors.New("invalid project role data view scope")
)

// DataViewScopeAll / DataViewScopePartial / DataViewScopeSelf are the project role
// list data view scopes (values align with the enterprise team work view scopes).
// 列表数据查看范围: all=全部成员, partial=部分成员, self=仅自己
const (
	DataViewScopeAll     = TeamWorkViewScopeAll
	DataViewScopePartial = TeamWorkViewScopePartial
	DataViewScopeSelf    = TeamWorkViewScopeSelf
)

// EnterpriseMemberModel describes enterprise member business data.
type EnterpriseMemberModel struct {
	Id            int
	Username      string
	Name          string
	Phone         string
	Email         string
	Avatar        string
	RoleId        int
	Status        int
	IsOwner       bool
	InvitationId  int
	DepartmentIds []int
	CreatedAt     int64
}

// ListEnterpriseMembersModel describes the enterprise member list use case.
type ListEnterpriseMembersModel struct {
	AdminId int
	Items   []EnterpriseMemberModel
}

// CreateEnterpriseMemberModel describes the create enterprise member use case.
type CreateEnterpriseMemberModel struct {
	AdminId        int
	OperatorUserId int
	Username       string
	Password       string
	Name           string
	Phone          string
	Email          string
	RoleId         int
	Avatar         string
	DepartmentIds  []int
	UserId         int
}

// UpdateEnterpriseMemberModel describes the update enterprise member use case.
type UpdateEnterpriseMemberModel struct {
	AdminId        int
	OperatorUserId int
	UserId         int
	Username       string
	Name           string
	Phone          string
	Email          string
	RoleId         int
	Avatar         string
	DepartmentIds  []int
	// UpdateDepartments false leaves department associations untouched (e.g. an empty department
	// column on Excel import keeps the member's current departments).
	UpdateDepartments bool
}

// TeamWorkTargetTypeUser / TeamWorkTargetTypeDept / TeamWorkTargetTypeRole are the entry
// types of a partial team work view scope.
const (
	TeamWorkTargetTypeUser = "user"
	TeamWorkTargetTypeDept = "dept"
	TeamWorkTargetTypeRole = "role"
)

// TeamWorkTargetModel describes one entry of a partial team work view scope.
type TeamWorkTargetModel struct {
	Type string
	Id   int
}

// EnterpriseRoleModel describes an enterprise role with its permissions and team work scope.
type EnterpriseRoleModel struct {
	Id                  int
	Name                string
	Description         string
	IsSystem            int
	Permissions         []string
	MemberCount         int
	TeamWorkViewScope   string
	TeamWorkViewTargets []TeamWorkTargetModel
}

// EnterpriseRoleBriefModel describes an enterprise role reduced to what member pickers render.
type EnterpriseRoleBriefModel struct {
	Id   int
	Name string
}

// ListEnterpriseMembers returns all active users under the tenant (admin_id isolated, soft-deleted excluded).
func ListEnterpriseMembers(model ListEnterpriseMembersModel) (ListEnterpriseMembersModel, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`deleted_at`, `0`},
	}).
		Field(`id, username, name, tel_number, email, avatar, role_id, status, is_owner, invitation_id, created_at`).
		Order(`id ASC`).
		Select()
	if err != nil {
		logs.Error("list enterprise members error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return ListEnterpriseMembersModel{}, err
	}
	model.Items = make([]EnterpriseMemberModel, 0, len(rows))
	departmentIds, err := ListUserDepartmentIds(model.AdminId)
	if err != nil {
		return ListEnterpriseMembersModel{}, err
	}
	for _, row := range rows {
		member, err := toEnterpriseMemberModel(row)
		if err != nil {
			logs.Error("convert enterprise member row error: %s", err.Error())
			return ListEnterpriseMembersModel{}, err
		}
		member.DepartmentIds = departmentIds[member.Id]
		model.Items = append(model.Items, member)
	}
	return model, nil
}

// CreateEnterpriseMember creates a tenant user and claims an optional pending avatar.
// 席位校验与用户插入在同一事务内（咨询锁串行），防止并发超卖套餐配额。
func CreateEnterpriseMember(model CreateEnterpriseMemberModel) (CreateEnterpriseMemberModel, error) {
	model.Username = validate.NormalizeEmail(model.Username)
	if !enterpriseRoleExists(model.AdminId, model.RoleId) {
		return CreateEnterpriseMemberModel{}, ErrEnterpriseRoleNotFound
	}
	model.Name = strings.TrimSpace(model.Name)
	model.Phone = strings.TrimSpace(model.Phone)
	model.Email = validate.NormalizeEmail(model.Email)
	exists, err := enterpriseUsernameExists(model.Username, 0)
	if err != nil {
		return CreateEnterpriseMemberModel{}, err
	}
	if exists {
		return CreateEnterpriseMemberModel{}, ErrUserUsernameExists
	}
	emailExists, err := UserEmailExist(model.Email, 0)
	if err != nil {
		return CreateEnterpriseMemberModel{}, err
	}
	if emailExists {
		return CreateEnterpriseMemberModel{}, ErrUserEmailExists
	}
	model.DepartmentIds, err = ValidateTenantDepartmentIds(model.AdminId, model.DepartmentIds)
	if err != nil {
		return CreateEnterpriseMemberModel{}, err
	}
	if err = EnsureDefaultPlan(model.AdminId); err != nil {
		return CreateEnterpriseMemberModel{}, err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return CreateEnterpriseMemberModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = CheckMemberQuotaTx(tx, model.AdminId); err != nil {
		return CreateEnterpriseMemberModel{}, err
	}
	userId, err := createUserTx(tx, CreateUserModel{
		Username: model.Username, Password: model.Password, Name: model.Name, Phone: model.Phone,
		Email: model.Email, AdminId: model.AdminId, RoleId: model.RoleId,
	}, 0)
	if err != nil {
		if isUserUsernameConflict(err) {
			return CreateEnterpriseMemberModel{}, ErrUserUsernameExists
		}
		if isUserEmailConflict(err) {
			return CreateEnterpriseMemberModel{}, ErrUserEmailExists
		}
		return CreateEnterpriseMemberModel{}, err
	}
	if err = setUserDepartmentsTx(tx, model.AdminId, userId, model.DepartmentIds); err != nil {
		return CreateEnterpriseMemberModel{}, err
	}
	if err = tx.Commit(); err != nil {
		return CreateEnterpriseMemberModel{}, err
	}
	if model.Avatar != `` {
		if _, err = ClaimPendingAvatar(model.AdminId, userId, model.Avatar); err != nil {
			if removeErr := purgeEnterpriseMember(model.AdminId, userId); removeErr != nil {
				logs.Error("rollback enterprise member error: %s", removeErr.Error())
			}
			return CreateEnterpriseMemberModel{}, err
		}
	}
	model.UserId = userId
	// Logged only once the member is fully created: an avatar failure above rolls the whole thing back.
	writeMemberLog(model.AdminId, model.OperatorUserId, userId, define.MemberLogActionCreate)
	return model, nil
}

// UpdateEnterpriseMember updates a tenant user's profile and enterprise role.
func UpdateEnterpriseMember(model UpdateEnterpriseMemberModel) error {
	model.Username = validate.NormalizeEmail(model.Username)
	model.Name = strings.TrimSpace(model.Name)
	model.Phone = strings.TrimSpace(model.Phone)
	model.Email = validate.NormalizeEmail(model.Email)
	userM := msql.Model(define.TableUsers, define.Postgres)
	user, err := userM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`id`, cast.ToString(model.UserId)},
		{`deleted_at`, `0`},
	}).
		Field(`id, username, avatar, role_id, is_owner`).Find()
	if err != nil {
		logs.Error("get enterprise member error: sql=%s, err=%s", userM.GetLastSql(), err.Error())
		return err
	}
	if len(user) == 0 {
		return ErrEnterpriseMemberNotFound
	}
	targetIsOwner := user[`is_owner`] == `1`
	if err = validateEnterpriseAdminUpdate(model, user[`username`], cast.ToInt(user[`role_id`]), targetIsOwner); err != nil {
		return err
	}
	if !enterpriseRoleExists(model.AdminId, model.RoleId) {
		return ErrEnterpriseRoleNotFound
	}
	emailExists, emailErr := UserEmailExist(model.Email, model.UserId)
	if emailErr != nil {
		return emailErr
	}
	if emailExists {
		return ErrUserEmailExists
	}
	if !targetIsOwner {
		exists, existsErr := enterpriseUsernameExists(model.Username, model.UserId)
		if existsErr != nil {
			return existsErr
		}
		if exists {
			return ErrUserUsernameExists
		}
	}
	if model.UpdateDepartments {
		model.DepartmentIds, err = ValidateTenantDepartmentIds(model.AdminId, model.DepartmentIds)
		if err != nil {
			return err
		}
	}
	data := msql.Datas{`name`: model.Name, `tel_number`: model.Phone, `email`: model.Email, `updated_at`: tool.Time2Int()}
	if !targetIsOwner {
		data[`username`] = model.Username
		data[`role_id`] = model.RoleId
	}
	if model.Avatar != `` {
		if !lib_storage.M.IsOwned(model.AdminId, lib_storage.BusinessEnterprise, model.Avatar) || !lib_storage.M.Exists(model.Avatar) {
			return ErrAvatarInvalid
		}
		data[`avatar`] = model.Avatar
	}
	updateM := msql.Model(define.TableUsers, define.Postgres)
	result, err := updateM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`id`, cast.ToString(model.UserId)},
		{`deleted_at`, `0`},
	}).Update(data)
	if err != nil {
		logs.Error("update enterprise member error: sql=%s, err=%s", updateM.GetLastSql(), err.Error())
		if isUserUsernameConflict(err) {
			return ErrUserUsernameExists
		}
		if isUserEmailConflict(err) {
			return ErrUserEmailExists
		}
		return err
	}
	if result == 0 {
		return ErrEnterpriseMemberNotFound
	}
	if model.UpdateDepartments {
		if err = SetUserDepartments(model.AdminId, model.UserId, model.DepartmentIds); err != nil {
			return err
		}
	}
	if model.Avatar != `` {
		removeOldAvatarFile(model.AdminId, model.UserId, user[`avatar`], model.Avatar)
	}
	return nil
}

// validateEnterpriseAdminUpdate protects the tenant owner row: only the owner themselves may edit it,
// and neither the username nor the enterprise role may change (the role follows ownership transfer).
func validateEnterpriseAdminUpdate(model UpdateEnterpriseMemberModel, currentUsername string, currentRoleId int, targetIsOwner bool) error {
	if !targetIsOwner {
		return nil
	}
	if model.OperatorUserId != model.UserId || model.Username != currentUsername || model.RoleId != currentRoleId {
		return ErrEnterpriseAdminProtected
	}
	return nil
}

func enterpriseUsernameExists(username string, excludedUserId int) (bool, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM users WHERE username=$1 AND ($2=0 OR id<>$2) LIMIT 1`, nil, username, excludedUserId)
	if err != nil {
		logs.Error("check enterprise member username error: %s", err.Error())
		return false, err
	}
	return len(rows) > 0, nil
}

func isUserUsernameConflict(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "idx_users_username"
}

func isUserEmailConflict(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "idx_users_email"
}

func toEnterpriseMemberModel(row msql.Params) (EnterpriseMemberModel, error) {
	id, err := strconv.Atoi(row[`id`])
	if err != nil {
		return EnterpriseMemberModel{}, fmt.Errorf("parse enterprise member id: %w", err)
	}
	roleId, err := strconv.Atoi(row[`role_id`])
	if err != nil {
		return EnterpriseMemberModel{}, fmt.Errorf("parse enterprise member role id: %w", err)
	}
	status, err := strconv.Atoi(row[`status`])
	if err != nil {
		return EnterpriseMemberModel{}, fmt.Errorf("parse enterprise member status: %w", err)
	}
	invitationId, err := strconv.Atoi(row[`invitation_id`])
	if err != nil {
		return EnterpriseMemberModel{}, fmt.Errorf("parse enterprise member invitation id: %w", err)
	}
	createdAt, err := strconv.ParseInt(row[`created_at`], 10, 64)
	if err != nil {
		return EnterpriseMemberModel{}, fmt.Errorf("parse enterprise member created at: %w", err)
	}
	return EnterpriseMemberModel{
		Id:           id,
		Username:     row[`username`],
		Name:         row[`name`],
		Phone:        row[`tel_number`],
		Email:        row[`email`],
		Avatar:       row[`avatar`],
		RoleId:       roleId,
		Status:       status,
		IsOwner:      row[`is_owner`] == `1`,
		InvitationId: invitationId,
		CreatedAt:    createdAt,
	}, nil
}

// UpdateEnterpriseMemberStatusModel describes the enable/disable enterprise member use case.
type UpdateEnterpriseMemberStatusModel struct {
	AdminId        int
	OperatorUserId int
	UserId         int
	Status         int
}

// UpdateEnterpriseMemberStatus enables or disables a tenant user.
// The tenant super admin can never be disabled, and nobody may disable their own account
// (doing so would revoke their own sessions and lock them out irrecoverably).
// Disabling revokes every session the member holds, so the change takes effect immediately.
func UpdateEnterpriseMemberStatus(model UpdateEnterpriseMemberStatusModel) error {
	if model.Status != define.MemberStatusEnabled && model.Status != define.MemberStatusDisabled {
		return ErrMemberStatusInvalid
	}
	if model.UserId == model.AdminId {
		return ErrEnterpriseAdminProtected
	}
	if model.UserId == model.OperatorUserId {
		return ErrMemberSelfStatusProtected
	}

	userM := msql.Model(define.TableUsers, define.Postgres)
	user, err := userM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`id`, cast.ToString(model.UserId)},
		{`deleted_at`, `0`},
	}).Field(`id, status`).Find()
	if err != nil {
		logs.Error("get enterprise member status error: sql=%s, err=%s", userM.GetLastSql(), err.Error())
		return err
	}
	if len(user) == 0 {
		return ErrEnterpriseMemberNotFound
	}
	// Already in the requested state: skip the write so the log does not collect no-op entries.
	if cast.ToInt(user[`status`]) == model.Status {
		return nil
	}

	updateM := msql.Model(define.TableUsers, define.Postgres)
	if _, err = updateM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`id`, cast.ToString(model.UserId)},
		{`deleted_at`, `0`},
	}).Update(msql.Datas{
		`status`:     model.Status,
		`updated_at`: tool.Time2Int(),
	}); err != nil {
		logs.Error("update enterprise member status error: sql=%s, err=%s", updateM.GetLastSql(), err.Error())
		return err
	}

	action := define.MemberLogActionEnable
	if model.Status == define.MemberStatusDisabled {
		action = define.MemberLogActionDisable
		// Revoke the member's sessions so the block applies to anyone already signed in.
		// The middleware reports these as an expired session on purpose: a revoked session
		// is not told why it died; the reason is given at the next login attempt.
		common.DeleteUserSessions(model.UserId, ``)
	}
	writeMemberLog(model.AdminId, model.OperatorUserId, model.UserId, action)
	return nil
}

// RemoveEnterpriseMemberModel describes the remove-enterprise-member use case.
// TransferOwnerToUserId is required when the member is the default project admin of any project;
// TransferItemsToUserId is optional and only set when the operator ticked the work item hand-over.
type RemoveEnterpriseMemberModel struct {
	AdminId               int
	OperatorUserId        int
	UserId                int
	TransferOwnerToUserId int
	TransferItemsToUserId int
	// IsRollback marks an internal cleanup of a member whose creation failed partway through.
	// Such a member never became visible to the tenant, so neither an archive entry nor an
	// operation log is written for them.
	IsRollback bool
}

// EnterpriseMemberRemovalPreviewModel describes what a member removal would drag along:
// the projects they own (their project admin role must be handed over) and how many unfinished
// work items are assigned to them (hand-over optional).
type EnterpriseMemberRemovalPreviewModel struct {
	AdminId         int
	UserId          int
	OwnedWorkspaces []WorkspaceBriefModel
	UnfinishedCount int
}

// PreviewEnterpriseMemberRemoval collects what the removal dialog needs to decide which
// hand-over rows to show.
func PreviewEnterpriseMemberRemoval(model EnterpriseMemberRemovalPreviewModel) (EnterpriseMemberRemovalPreviewModel, error) {
	owned, err := ListOwnedWorkspaces(model.AdminId, model.UserId)
	if err != nil {
		return EnterpriseMemberRemovalPreviewModel{}, err
	}
	count, err := CountUnfinishedAssignedItems(model.AdminId, 0, model.UserId)
	if err != nil {
		return EnterpriseMemberRemovalPreviewModel{}, err
	}
	model.OwnedWorkspaces = owned
	model.UnfinishedCount = count
	return model, nil
}

// RemoveEnterpriseMember soft-deletes a tenant user and drops their workspace memberships.
// The users row stays so historical work items keep rendering the member's name and avatar;
// username and email are cleared to release the global account and email uniqueness
// (both unique indexes skip empty values).
// Ownership of the projects they run and, optionally, their unfinished work items are handed over
// first, all inside one transaction so a failure leaves neither half applied.
// Yun still archives into removed_users and writes a member log so existing LEFT JOIN
// paths and the operation timeline keep working.
func RemoveEnterpriseMember(model RemoveEnterpriseMemberModel) error {
	adminId, userId := model.AdminId, model.UserId
	if IsTenantOwner(adminId, userId) {
		return ErrEnterpriseAdminProtected
	}

	ownedWorkspaces, err := ListOwnedWorkspaces(adminId, userId)
	if err != nil {
		return err
	}
	if len(ownedWorkspaces) > 0 {
		if err = validateTransferTarget(adminId, userId, model.TransferOwnerToUserId); err != nil {
			return err
		}
	}
	if model.TransferItemsToUserId > 0 {
		if err = validateTransferTarget(adminId, userId, model.TransferItemsToUserId); err != nil {
			return err
		}
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin remove enterprise member error: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if !model.IsRollback {
		if err = archiveRemovedUser(tx, adminId, userId); err != nil {
			return err
		}
	}
	if err = RefreshAutomationCreatorSnapshots(tx, adminId, userId); err != nil {
		logs.Error("refresh automation creator snapshot failed: %s", err.Error())
		return err
	}

	// Hand over project ownership before the membership rows are dropped.
	for _, workspace := range ownedWorkspaces {
		if err = transferWorkspaceOwnership(tx, adminId, workspace.Id, userId, model.TransferOwnerToUserId, false); err != nil {
			return err
		}
	}

	if model.TransferItemsToUserId > 0 {
		workspaceIds, transferErr := transferUnfinishedAssignedItems(tx, adminId, 0, userId, model.TransferItemsToUserId, model.OperatorUserId)
		if transferErr != nil {
			return transferErr
		}
		// The receiver must be able to open the items they just inherited.
		for _, wsId := range workspaceIds {
			if err = ensureWorkspaceMembershipIfMissing(tx, adminId, wsId, model.TransferItemsToUserId); err != nil {
				return err
			}
		}
	}

	if err = removeEnterpriseMemberRelations(tx, adminId, userId); err != nil {
		return err
	}

	now := tool.Time2Int()
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableUsers+`
		SET username = '', email = '', status = 0, deleted_at = $1, updated_at = $1
		WHERE admin_id = $2 AND id = $3 AND deleted_at = 0`, tx, now, adminId, userId); err != nil {
		logs.Error("soft delete enterprise member error: %s", err.Error())
		return err
	}

	if err = tx.Commit(); err != nil {
		logs.Error("commit remove enterprise member error: %s", err.Error())
		return err
	}

	// Member removed; immediately revoke all their login sessions.
	common.DeleteUserSessions(userId, ``)
	if !model.IsRollback {
		writeMemberLog(adminId, model.OperatorUserId, userId, define.MemberLogActionRemove)
	}
	return nil
}

// validateTransferTarget checks a hand-over receiver: not the member being removed, and an
// enabled, non-removed user of the same tenant.
func validateTransferTarget(adminId, removedUserId, targetUserId int) error {
	if targetUserId <= 0 || targetUserId == removedUserId {
		return ErrOwnerTransferTarget
	}
	valid, err := transferableTenantMember(adminId, targetUserId)
	if err != nil {
		return err
	}
	if !valid {
		return ErrOwnerTransferTarget
	}
	return nil
}

// purgeEnterpriseMember hard-deletes a tenant user; used to roll back a member that was just created,
// so a failed creation leaves no soft-deleted row behind.
func purgeEnterpriseMember(adminId int, userId int) error {
	if IsTenantOwner(adminId, userId) {
		return ErrEnterpriseAdminProtected
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin purge enterprise member error: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err = removeEnterpriseMemberRelations(tx, adminId, userId); err != nil {
		return err
	}

	_, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableUsers+`
		WHERE admin_id = $1 AND id = $2`, tx, adminId, userId)
	if err != nil {
		logs.Error("delete enterprise member error: %s", err.Error())
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit purge enterprise member error: %s", err.Error())
		return err
	}

	common.DeleteUserSessions(userId, ``)
	return nil
}

// ListRemovedTenantMembers returns the tenant's soft-deleted members as workspace member models,
// carrying only what history rendering needs (user id, name, avatar) with Removed set.
func ListRemovedTenantMembers(adminId int) ([]WorkspaceMemberModel, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	rows, err := m.Where(`admin_id`, cast.ToString(adminId)).
		Where(`deleted_at`, `>`, `0`).
		Field(`id, name, avatar`).Order(`id ASC`).Select()
	if err != nil {
		logs.Error("list removed tenant members error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	items := make([]WorkspaceMemberModel, 0, len(rows))
	for _, row := range rows {
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

// tenantUserIds returns the subset of ids that belong to the tenant, whether or not the user is
// still an active member. Callers use it to keep historical references (work item assignees...)
// valid: a member can leave in two ways — removed from the enterprise (users row soft-deleted) or
// merely dropped from one project (users row untouched) — and both must stay renderable and
// editable. Guarding against "assign a departed member again" is the caller's job, by only passing
// ids the work item already carries.
func tenantUserIds(adminId int, ids []int) (map[int]bool, error) {
	result := make(map[int]bool, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM users
		WHERE admin_id = $1 AND id = ANY($2)`, nil, adminId, pq.Array(ids))
	if err != nil {
		logs.Error("list tenant users error: %s", err.Error())
		return nil, err
	}
	for _, row := range rows {
		result[cast.ToInt(row[`id`])] = true
	}
	return result, nil
}

// removeEnterpriseMemberRelations drops every tenant relation of a member (team work settings,
// contact preferences, role scopes, pipeline memberships, dashboard relations, workspace memberships and departments).
// tx may be nil.
func removeEnterpriseMemberRelations(tx *sql.Tx, adminId int, userId int) error {
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM user_team_work_member_settings
		WHERE admin_id = $1 AND (user_id = $2 OR member_user_id = $2)`, tx, adminId, userId); err != nil {
		logs.Error("delete enterprise member team work settings error: %s", err.Error())
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM user_contact_preferences
		WHERE admin_id = $1 AND (user_id = $2 OR contact_user_id = $2)`, tx, adminId, userId); err != nil {
		logs.Error("delete enterprise member contact preferences error: %s", err.Error())
		return err
	}

	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableRoleTeamWorkMembers+`
		WHERE admin_id = $1 AND target_type = $2 AND target_id = $3`,
		tx, adminId, TeamWorkTargetTypeUser, userId); err != nil {
		logs.Error("delete enterprise member team work scopes error: %s", err.Error())
		return err
	}

	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableRoleDataViewMembers+`
		WHERE admin_id = $1 AND user_id = $2`, tx, adminId, userId); err != nil {
		logs.Error("delete enterprise member project data view scopes error: %s", err.Error())
		return err
	}

	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TablePipelineMember+`
		WHERE admin_id = $1 AND user_id = $2`, tx, adminId, userId); err != nil {
		logs.Error("delete enterprise member pipeline memberships error: %s", err.Error())
		return err
	}

	workspaceRows, err := msql.RawValues(define.Postgres, `SELECT DISTINCT workspace_id FROM `+define.TableWorkspaceMembers+`
		WHERE admin_id = $1 AND user_id = $2`, tx, adminId, userId)
	if err != nil {
		logs.Error("list enterprise member workspaces failed, err:%s", err.Error())
		return err
	}
	for _, row := range workspaceRows {
		if err = deleteDashboardMemberRelationsTx(tx, adminId, cast.ToInt(row[`workspace_id`]), userId); err != nil {
			return err
		}
	}

	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableWorkspaceMembers+`
		WHERE admin_id = $1 AND user_id = $2`, tx, adminId, userId); err != nil {
		logs.Error("delete enterprise member workspace memberships error: %s", err.Error())
		return err
	}

	return RemoveUserDepartments(tx, adminId, userId)
}

// archiveRemovedUser snapshots display info into removed_users. Soft-delete keeps the users
// row, but yun historical queries still LEFT JOIN this table; dual-write keeps those paths
// working for newly removed members. Upsert is defensive if the same id is archived twice.
func archiveRemovedUser(tx *sql.Tx, adminId, userId int) error {
	users, err := msql.RawValues(define.Postgres, `SELECT name, avatar FROM `+define.TableUsers+`
		WHERE admin_id = $1 AND id = $2`, tx, adminId, userId)
	if err != nil {
		logs.Error("read member for archive error: %s", err.Error())
		return err
	}
	if len(users) == 0 {
		return ErrEnterpriseMemberNotFound
	}

	if _, err = msql.RawExec(define.Postgres, `INSERT INTO removed_users (admin_id, user_id, name, avatar, removed_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (admin_id, user_id) DO UPDATE SET name = EXCLUDED.name, avatar = EXCLUDED.avatar, removed_at = EXCLUDED.removed_at`,
		tx, adminId, userId, users[0][`name`], users[0][`avatar`], tool.Time2Int()); err != nil {
		logs.Error("archive removed user error: %s", err.Error())
		return err
	}
	return nil
}

// ResetEnterpriseMemberPasswordModel describes the admin reset-member-password use case.
type ResetEnterpriseMemberPasswordModel struct {
	AdminId        int
	OperatorUserId int
	TargetUserId   int
	NewPassword    string
}

// ResetEnterpriseMemberPassword resets a tenant user's password and kills all their sessions.
func ResetEnterpriseMemberPassword(model ResetEnterpriseMemberPasswordModel) error {
	if IsTenantOwner(model.AdminId, model.TargetUserId) && model.OperatorUserId != model.TargetUserId {
		return ErrEnterpriseAdminProtected
	}
	return updateUserPassword(model.AdminId, model.TargetUserId, model.NewPassword, ``)
}

// ======================== Enterprise Role Management ========================

// ListEnterpriseRoles returns all enterprise-level roles for the given tenant (scope='enterprise', admin_id isolated).
func ListEnterpriseRoles(adminId int) ([]EnterpriseRoleModel, error) {
	m := msql.Model(define.TableRoles, define.Postgres)
	list, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`scope`, permission.ScopeEnterprise},
	}).Field(`id, name, description, is_system, team_work_view_scope`).
		Order(`id ASC`).
		Select()
	if err != nil {
		logs.Error("list enterprise roles error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}

	result := make([]EnterpriseRoleModel, 0, len(list))
	for _, role := range list {
		roleId := cast.ToInt(role[`id`])
		count, err := countEnterpriseRoleMembers(adminId, roleId)
		if err != nil {
			return nil, err
		}
		viewTargets, err := getRoleTeamWorkTargets(adminId, roleId)
		if err != nil {
			return nil, err
		}
		result = append(result, EnterpriseRoleModel{
			Id:                  roleId,
			Name:                role[`name`],
			Description:         role[`description`],
			IsSystem:            cast.ToInt(role[`is_system`]),
			Permissions:         getRolePermissions(adminId, roleId),
			MemberCount:         count,
			TeamWorkViewScope:   role[`team_work_view_scope`],
			TeamWorkViewTargets: viewTargets,
		})
	}
	return result, nil
}

// ListEnterpriseRoleBriefs returns the tenant's enterprise roles with id and name only
// (scope='enterprise', admin_id isolated), skipping the per-role permission and member counting
// that ListEnterpriseRoles performs for the role configuration page.
func ListEnterpriseRoleBriefs(adminId int) ([]EnterpriseRoleBriefModel, error) {
	m := msql.Model(define.TableRoles, define.Postgres)
	list, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`scope`, permission.ScopeEnterprise},
	}).Field(`id, name`).Order(`id ASC`).Select()
	if err != nil {
		logs.Error("list enterprise role briefs error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}

	result := make([]EnterpriseRoleBriefModel, 0, len(list))
	for _, role := range list {
		result = append(result, EnterpriseRoleBriefModel{Id: cast.ToInt(role[`id`]), Name: role[`name`]})
	}
	return result, nil
}

// CreateEnterpriseRole creates a custom enterprise role and assigns permissions.
func CreateEnterpriseRole(adminId int, name string, description string, permKeys []string) (int, error) {
	if enterpriseRoleNameExists(adminId, name) {
		return 0, ErrEnterpriseRoleNameExists
	}

	now := tool.Time2Int()
	m := msql.Model(define.TableRoles, define.Postgres)
	id, err := m.Insert(msql.Datas{
		`admin_id`:             adminId,
		`workspace_id`:         0,
		`name`:                 name,
		`description`:          description,
		`is_system`:            0,
		`scope`:                permission.ScopeEnterprise,
		`team_work_view_scope`: TeamWorkViewScopeSelf,
		`created_at`:           now,
		`updated_at`:           now,
	}, `id`)
	if err != nil {
		logs.Error("create enterprise role error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return 0, err
	}

	roleId := cast.ToInt(id)
	if err := SetRolePermissions(adminId, roleId, filterEnterprisePermissionKeys(permKeys)); err != nil {
		return 0, err
	}
	return roleId, nil
}

// UpdateEnterpriseRolePermissions replaces permissions for a custom enterprise role.
func UpdateEnterpriseRolePermissions(adminId int, roleId int, permKeys []string) error {
	role, err := getEnterpriseRole(adminId, roleId)
	if err != nil {
		return err
	}
	if len(role) == 0 {
		return ErrEnterpriseRoleNotFound
	}
	if cast.ToInt(role[`is_system`]) == 1 {
		return ErrEnterpriseRoleSystem
	}
	return SetRolePermissions(adminId, roleId, filterEnterprisePermissionKeys(permKeys))
}

// UpdateEnterpriseRoleBasic updates the name and description of a custom enterprise role.
// The role must belong to the current tenant (admin_id isolated); system preset roles stay
// immutable and names already used by another role of the tenant are rejected.
func UpdateEnterpriseRoleBasic(req define.UpdateRoleBasicReq) error {
	role, err := getEnterpriseRole(req.AdminID, req.RoleID)
	if err != nil {
		return err
	}
	if len(role) == 0 {
		return ErrEnterpriseRoleNotFound
	}
	if cast.ToInt(role[`is_system`]) == 1 {
		return ErrEnterpriseRoleSystem
	}
	if enterpriseRoleNameConflict(req.AdminID, req.RoleID, req.Name) {
		return ErrEnterpriseRoleNameExists
	}

	m := msql.Model(define.TableRoles, define.Postgres)
	if _, err = m.Where2([][]string{
		{`admin_id`, cast.ToString(req.AdminID)},
		{`id`, cast.ToString(req.RoleID)},
		{`scope`, permission.ScopeEnterprise},
	}).Update(msql.Datas{
		`name`:        req.Name,
		`description`: req.Description,
		`updated_at`:  tool.Time2Int(),
	}); err != nil {
		logs.Error("update enterprise role error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// enterpriseRoleNameConflict reports whether another enterprise role of the tenant already uses the name.
func enterpriseRoleNameConflict(adminId int, roleId int, name string) bool {
	m := msql.Model(define.TableRoles, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`name`, name},
		{`scope`, permission.ScopeEnterprise},
	}).Value(`id`)
	if err != nil {
		logs.Error("check enterprise role name error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return false
	}
	if id == `` {
		return false
	}
	return cast.ToInt(id) != roleId
}

// UpdateEnterpriseRoleTeamWorkScope Update the team work viewing scope of custom enterprise roles.
func UpdateEnterpriseRoleTeamWorkScope(adminId int, roleId int, viewScope string, targets []TeamWorkTargetModel) error {
	role, err := getEnterpriseRole(adminId, roleId)
	if err != nil {
		return err
	}
	if len(role) == 0 {
		return ErrEnterpriseRoleNotFound
	}
	if cast.ToInt(role[`is_system`]) == 1 {
		return ErrEnterpriseRoleSystem
	}
	if viewScope != TeamWorkViewScopeAll && viewScope != TeamWorkViewScopePartial && viewScope != TeamWorkViewScopeSelf {
		return ErrEnterpriseTeamWorkScopeInvalid
	}

	targets, err = normalizeTeamWorkTargets(adminId, roleId, viewScope, targets)
	if err != nil {
		return err
	}

	scopeM := msql.Model(define.TableRoleTeamWorkMembers, define.Postgres)
	if _, err = scopeM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
	}).Delete(); err != nil {
		logs.Error("delete enterprise role team work members error: sql=%s, err=%s", scopeM.GetLastSql(), err.Error())
		return err
	}

	now := tool.Time2Int()
	for _, target := range targets {
		insertM := msql.Model(define.TableRoleTeamWorkMembers, define.Postgres)
		if _, err = insertM.Insert(msql.Datas{
			`admin_id`:    adminId,
			`role_id`:     roleId,
			`target_type`: target.Type,
			`target_id`:   target.Id,
			`created_at`:  now,
		}); err != nil {
			logs.Error("insert enterprise role team work target error: sql=%s, err=%s", insertM.GetLastSql(), err.Error())
			return err
		}
	}

	roleM := msql.Model(define.TableRoles, define.Postgres)
	_, err = roleM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(roleId)},
		{`scope`, permission.ScopeEnterprise},
	}).Update(msql.Datas{
		`team_work_view_scope`: viewScope,
		`updated_at`:           now,
	})
	if err != nil {
		logs.Error("update enterprise role team work scope error: sql=%s, err=%s", roleM.GetLastSql(), err.Error())
	}
	return err
}

// DeleteEnterpriseRole deletes a custom enterprise role if no users are assigned.
func DeleteEnterpriseRole(adminId int, roleId int) error {
	role, err := getEnterpriseRole(adminId, roleId)
	if err != nil {
		return err
	}
	if len(role) == 0 {
		return ErrEnterpriseRoleNotFound
	}
	if cast.ToInt(role[`is_system`]) == 1 {
		return ErrEnterpriseRoleSystem
	}

	count, err := countEnterpriseRoleMembers(adminId, roleId)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrEnterpriseRoleInUse
	}

	permM := msql.Model(define.TableRolePermissions, define.Postgres)
	if _, err := permM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
	}).Delete(); err != nil {
		logs.Error("delete enterprise role permissions error: sql=%s, err=%s", permM.GetLastSql(), err.Error())
		return err
	}

	scopeM := msql.Model(define.TableRoleTeamWorkMembers, define.Postgres)
	if _, err := scopeM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
	}).Delete(); err != nil {
		logs.Error("delete enterprise role team work members error: sql=%s, err=%s", scopeM.GetLastSql(), err.Error())
		return err
	}

	roleM := msql.Model(define.TableRoles, define.Postgres)
	_, err = roleM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(roleId)},
		{`scope`, permission.ScopeEnterprise},
	}).Delete()
	if err != nil {
		logs.Error("delete enterprise role error: sql=%s, err=%s", roleM.GetLastSql(), err.Error())
	}
	return err
}

// ProjectRoleInfo is project role information (including the permission list), used in the project role config page.
type ProjectRoleInfo struct {
	Id                int      `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	IsSystem          int      `json:"is_system"`
	IsEditable        bool     `json:"is_editable"`
	Permissions       []string `json:"permissions"`
	MemberCount       int      `json:"member_count"`
	DataViewScope     string   `json:"data_view_scope"`
	DataViewMemberIds []int    `json:"data_view_member_ids"`
}

// ListProjectRoles returns all project roles for a specific workspace (admin_id isolated).
// Each role carries its permission key list, for the frontend project role config page to check/display.
func ListProjectRoles(adminId int, wsId string) ([]ProjectRoleInfo, error) {
	m := msql.Model(define.TableRoles, define.Postgres)
	list, err := m.Where2([][]string{
		{`workspace_id`, wsId},
		{`scope`, permission.ScopeProject},
		{`admin_id`, cast.ToString(adminId)},
	}).Field(`id, name, description, is_system, data_view_scope`).
		Order(`id ASC`).
		Select()
	if err != nil {
		logs.Error("list project roles error: %s", err.Error())
		return nil, err
	}

	result := make([]ProjectRoleInfo, 0, len(list))
	for _, role := range list {
		roleId := cast.ToInt(role[`id`])
		count, _ := countRoleMembers(adminId, roleId)
		viewMemberIds, err := getRoleDataViewMemberIds(adminId, roleId)
		if err != nil {
			return nil, err
		}
		viewScope := role[`data_view_scope`]
		if viewScope == `` {
			viewScope = DataViewScopeSelf
		}
		result = append(result, ProjectRoleInfo{
			Id:          roleId,
			Name:        role[`name`],
			Description: role[`description`],
			IsSystem:    cast.ToInt(role[`is_system`]),
			// Custom roles and the preset project member role are editable; project admin stays read-only.
			IsEditable:        cast.ToInt(role[`is_system`]) == 0 || isEditableSystemProjectRole(role),
			Permissions:       getRolePermissions(adminId, roleId),
			MemberCount:       count,
			DataViewScope:     viewScope,
			DataViewMemberIds: viewMemberIds,
		})
	}
	return result, nil
}

func countRoleMembers(adminId int, roleId int) (int, error) {
	m := msql.Model(define.TableWorkspaceMembers, define.Postgres)
	count, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
	}).Count()
	return count, err
}

// CreateProjectRole creates a custom project role for a workspace and assigns permissions.
// workspace_id isolated, is_system=0 (custom role), only accepts permission keys within the project scope.
func CreateProjectRole(adminId int, wsId string, name string, description string, permKeys []string) (int, error) {
	if projectRoleNameExists(adminId, wsId, name) {
		return 0, ErrProjectRoleNameExists
	}

	now := tool.Time2Int()
	m := msql.Model(define.TableRoles, define.Postgres)
	id, err := m.Insert(msql.Datas{
		`admin_id`:        adminId,
		`workspace_id`:    wsId,
		`name`:            name,
		`description`:     description,
		`is_system`:       0,
		`scope`:           permission.ScopeProject,
		`data_view_scope`: DataViewScopeSelf,
		`created_at`:      now,
		`updated_at`:      now,
	}, `id`)
	if err != nil {
		logs.Error("create project role error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return 0, err
	}

	roleId := cast.ToInt(id)
	if err := SetRolePermissions(adminId, roleId, filterProjectPermissionKeys(permKeys)); err != nil {
		return 0, err
	}
	return roleId, nil
}

func projectRoleNameExists(adminId int, wsId string, name string) bool {
	m := msql.Model(define.TableRoles, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`name`, name},
		{`workspace_id`, wsId},
		{`scope`, permission.ScopeProject},
	}).Value(`id`)
	if err != nil {
		logs.Error("check project role name error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return false
	}
	return len(id) > 0
}

// isEditableSystemProjectRole reports whether a system preset project role allows editing.
// Only the preset project member role is editable; other system roles (project admin) stay read-only.
func isEditableSystemProjectRole(role msql.Params) bool {
	return role[`name`] == lib_i18n.WrapKey("perm_role_project_member")
}

// UpdateProjectRolePermissions replaces permissions for a project role.
// Custom roles and the editable preset project member role are accepted; other system preset roles
// (project admin) stay immutable; only permission keys within the project scope are written.
func UpdateProjectRolePermissions(adminId int, wsId string, roleId int, permKeys []string) error {
	role, err := getProjectRole(adminId, wsId, roleId)
	if err != nil {
		return err
	}
	if len(role) == 0 {
		return ErrProjectRoleNotFound
	}
	if cast.ToInt(role[`is_system`]) == 1 && !isEditableSystemProjectRole(role) {
		return ErrProjectRoleSystem
	}
	return SetRolePermissions(adminId, roleId, filterProjectPermissionKeys(permKeys))
}

// UpdateProjectRoleBasic updates the name and description of a project role.
// The role must belong to the current tenant + workspace; system preset roles stay immutable
// (their names are i18n keys, not editable text) and names already used by another role
// of the workspace are rejected.
func UpdateProjectRoleBasic(req define.UpdateRoleBasicReq) error {
	role, err := getProjectRole(req.AdminID, req.WorkspaceID, req.RoleID)
	if err != nil {
		return err
	}
	if len(role) == 0 {
		return ErrProjectRoleNotFound
	}
	if cast.ToInt(role[`is_system`]) == 1 {
		return ErrProjectRoleSystem
	}
	if projectRoleNameConflict(req.AdminID, req.WorkspaceID, req.RoleID, req.Name) {
		return ErrProjectRoleNameExists
	}

	m := msql.Model(define.TableRoles, define.Postgres)
	if _, err = m.Where2([][]string{
		{`admin_id`, cast.ToString(req.AdminID)},
		{`id`, cast.ToString(req.RoleID)},
		{`workspace_id`, req.WorkspaceID},
		{`scope`, permission.ScopeProject},
	}).Update(msql.Datas{
		`name`:        req.Name,
		`description`: req.Description,
		`updated_at`:  tool.Time2Int(),
	}); err != nil {
		logs.Error("update project role error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// projectRoleNameConflict reports whether another role of the workspace already uses the name.
func projectRoleNameConflict(adminId int, wsId string, roleId int, name string) bool {
	m := msql.Model(define.TableRoles, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`name`, name},
		{`workspace_id`, wsId},
		{`scope`, permission.ScopeProject},
	}).Value(`id`)
	if err != nil {
		logs.Error("check project role name error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return false
	}
	if id == `` {
		return false
	}
	return cast.ToInt(id) != roleId
}

// UpdateProjectRoleDataViewScope updates the list data view scope of a project role.
// The role must belong to the current tenant + workspace; system preset roles stay immutable
// except the editable preset project member role;
// for partial scope, the member IDs are validated against the workspace members.
func UpdateProjectRoleDataViewScope(adminId int, wsId string, roleId int, viewScope string, memberIds []int) error {
	role, err := getProjectRole(adminId, wsId, roleId)
	if err != nil {
		return err
	}
	if len(role) == 0 {
		return ErrProjectRoleNotFound
	}
	if cast.ToInt(role[`is_system`]) == 1 && !isEditableSystemProjectRole(role) {
		return ErrProjectRoleSystem
	}
	if viewScope != DataViewScopeAll && viewScope != DataViewScopePartial && viewScope != DataViewScopeSelf {
		return ErrProjectRoleDataViewScopeInvalid
	}

	memberIds, err = normalizeProjectDataViewMemberIds(adminId, wsId, viewScope, memberIds)
	if err != nil {
		return err
	}

	scopeM := msql.Model(define.TableRoleDataViewMembers, define.Postgres)
	if _, err = scopeM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
	}).Delete(); err != nil {
		logs.Error("delete project role data view members error: sql=%s, err=%s", scopeM.GetLastSql(), err.Error())
		return err
	}

	now := tool.Time2Int()
	for _, userId := range memberIds {
		insertM := msql.Model(define.TableRoleDataViewMembers, define.Postgres)
		if _, err = insertM.Insert(msql.Datas{
			`admin_id`:   adminId,
			`role_id`:    roleId,
			`user_id`:    userId,
			`created_at`: now,
		}); err != nil {
			logs.Error("insert project role data view member error: sql=%s, err=%s", insertM.GetLastSql(), err.Error())
			return err
		}
	}

	roleM := msql.Model(define.TableRoles, define.Postgres)
	_, err = roleM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(roleId)},
		{`workspace_id`, wsId},
		{`scope`, permission.ScopeProject},
	}).Update(msql.Datas{
		`data_view_scope`: viewScope,
		`updated_at`:      now,
	})
	if err != nil {
		logs.Error("update project role data view scope error: sql=%s, err=%s", roleM.GetLastSql(), err.Error())
	}
	return err
}

// getRoleDataViewMemberIds returns the member user IDs configured for the project role's partial data view scope.
func getRoleDataViewMemberIds(adminId int, roleId int) ([]int, error) {
	return listRoleDataViewMemberIds(adminId, roleId)
}

// normalizeProjectDataViewMemberIds validates and dedupes member IDs for the partial data view scope.
// Only workspace members (admin_id + workspace_id isolated) are accepted; an empty result is rejected.
func normalizeProjectDataViewMemberIds(adminId int, wsId string, viewScope string, memberIds []int) ([]int, error) {
	if viewScope != DataViewScopePartial {
		return nil, nil
	}

	result := make([]int, 0, len(memberIds))
	seen := make(map[int]bool, len(memberIds))
	for _, userId := range memberIds {
		if userId <= 0 || seen[userId] {
			continue
		}
		m := msql.Model(define.TableWorkspaceMembers, define.Postgres)
		member, err := m.Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`workspace_id`, wsId},
			{`user_id`, cast.ToString(userId)},
		}).Field(`id`).Find()
		if err != nil {
			logs.Error("validate project data view member error: sql=%s, err=%s", m.GetLastSql(), err.Error())
			return nil, err
		}
		if len(member) == 0 {
			return nil, ErrProjectRoleDataViewScopeInvalid
		}
		seen[userId] = true
		result = append(result, userId)
	}
	if len(result) == 0 {
		return nil, ErrProjectRoleDataViewScopeInvalid
	}
	return result, nil
}

func getProjectRole(adminId int, wsId string, roleId int) (msql.Params, error) {
	m := msql.Model(define.TableRoles, define.Postgres)
	role, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(roleId)},
		{`workspace_id`, wsId},
		{`scope`, permission.ScopeProject},
	}).Find()
	if err != nil {
		logs.Error("get project role error: sql=%s, err=%s", m.GetLastSql(), err.Error())
	}
	return role, err
}

// DeleteProjectRole deletes a custom project role if no workspace members are assigned.
// Validate that the role belongs to the current tenant + workspace and is not a system preset role.
func DeleteProjectRole(adminId int, wsId string, roleId int) error {
	role, err := getProjectRole(adminId, wsId, roleId)
	if err != nil {
		return err
	}
	if len(role) == 0 {
		return ErrProjectRoleNotFound
	}
	if cast.ToInt(role[`is_system`]) == 1 {
		return ErrProjectRoleSystem
	}

	count, err := countProjectRoleMembers(adminId, wsId, roleId)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrProjectRoleInUse
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin delete project role error: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableRolePermissions+`
		WHERE admin_id = $1 AND role_id = $2`, tx, adminId, roleId); err != nil {
		logs.Error("delete project role permissions error: %s", err.Error())
		return err
	}

	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableRoleDataViewMembers+`
		WHERE admin_id = $1 AND role_id = $2`, tx, adminId, roleId); err != nil {
		logs.Error("delete project role data view members error: %s", err.Error())
		return err
	}

	if err = deleteDashboardRoleRelationsTx(tx, adminId, cast.ToInt(wsId), roleId); err != nil {
		return err
	}

	result, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableRoles+`
		WHERE admin_id = $1 AND id = $2 AND workspace_id = $3 AND scope = $4
		AND NOT EXISTS (
			SELECT 1 FROM `+define.TableWorkspaceMembers+`
			WHERE admin_id = $1 AND workspace_id = $3 AND role_id = $2
		)`, tx, adminId, roleId, cast.ToInt(wsId), permission.ScopeProject)
	if err != nil {
		logs.Error("delete project role error: %s", err.Error())
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		logs.Error("read deleted project role count error: %s", err.Error())
		return err
	}
	if affected == 0 {
		return ErrProjectRoleInUse
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit delete project role error: %s", err.Error())
	}
	return err
}

func countProjectRoleMembers(adminId int, wsId string, roleId int) (int, error) {
	m := msql.Model(define.TableWorkspaceMembers, define.Postgres)
	count, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`role_id`, cast.ToString(roleId)},
	}).Count()
	if err != nil {
		logs.Error("count project role members error: sql=%s, err=%s", m.GetLastSql(), err.Error())
	}
	return count, err
}

func getEnterpriseRole(adminId int, roleId int) (msql.Params, error) {
	m := msql.Model(define.TableRoles, define.Postgres)
	role, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(roleId)},
		{`scope`, permission.ScopeEnterprise},
	}).Find()
	if err != nil {
		logs.Error("get enterprise role error: sql=%s, err=%s", m.GetLastSql(), err.Error())
	}
	return role, err
}

// EnterpriseRoleExists checks whether an enterprise role belongs to the current tenant.
func EnterpriseRoleExists(adminId int, roleId int) bool {
	role, err := getEnterpriseRole(adminId, roleId)
	return err == nil && len(role) > 0
}

func enterpriseRoleExists(adminId int, roleId int) bool {
	return EnterpriseRoleExists(adminId, roleId)
}

func enterpriseRoleNameExists(adminId int, name string) bool {
	m := msql.Model(define.TableRoles, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`name`, name},
		{`scope`, permission.ScopeEnterprise},
	}).Value(`id`)
	if err != nil {
		logs.Error("check enterprise role name error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return false
	}
	return len(id) > 0
}

func countEnterpriseRoleMembers(adminId int, roleId int) (int, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	count, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
		{`deleted_at`, `0`},
	}).Count()
	if err != nil {
		logs.Error("count enterprise role members error: sql=%s, err=%s", m.GetLastSql(), err.Error())
	}
	return count, err
}

func filterEnterprisePermissionKeys(keys []string) []string {
	result := make([]string, 0, len(keys))
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if seen[key] {
			continue
		}
		perm, ok := permission.GetPermission(key)
		if !ok {
			continue
		}
		if !enterprisePermGroups[perm.Group] {
			continue
		}
		seen[key] = true
		result = append(result, key)
	}
	return result
}

func getRoleTeamWorkTargets(adminId int, roleId int) ([]TeamWorkTargetModel, error) {
	m := msql.Model(define.TableRoleTeamWorkMembers, define.Postgres)
	list, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
	}).Field(`target_type, target_id`).Order(`id ASC`).Select()
	if err != nil {
		logs.Error("list enterprise role team work targets error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	result := make([]TeamWorkTargetModel, 0, len(list))
	for _, row := range list {
		result = append(result, TeamWorkTargetModel{Type: row[`target_type`], Id: cast.ToInt(row[`target_id`])})
	}
	return result, nil
}

// normalizeTeamWorkTargets dedupes the partial-scope entries and validates that every
// referenced member, department and enterprise role exists in the tenant. The role
// being configured cannot reference itself.
func normalizeTeamWorkTargets(adminId int, roleId int, viewScope string, targets []TeamWorkTargetModel) ([]TeamWorkTargetModel, error) {
	if viewScope != TeamWorkViewScopePartial {
		return nil, nil
	}

	result := make([]TeamWorkTargetModel, 0, len(targets))
	seen := make(map[TeamWorkTargetModel]bool, len(targets))
	for _, target := range targets {
		if target.Id <= 0 || seen[target] {
			continue
		}
		switch target.Type {
		case TeamWorkTargetTypeUser:
			m := msql.Model(define.TableUsers, define.Postgres)
			member, err := m.Where2([][]string{
				{`admin_id`, cast.ToString(adminId)},
				{`id`, cast.ToString(target.Id)},
				{`deleted_at`, `0`},
			}).Field(`id`).Find()
			if err != nil {
				logs.Error("validate team work member error: sql=%s, err=%s", m.GetLastSql(), err.Error())
				return nil, err
			}
			if len(member) == 0 {
				return nil, ErrEnterpriseTeamWorkScopeInvalid
			}
		case TeamWorkTargetTypeDept:
			dept, err := getDepartment(adminId, target.Id)
			if err != nil {
				return nil, err
			}
			if len(dept) == 0 {
				return nil, ErrEnterpriseTeamWorkScopeInvalid
			}
		case TeamWorkTargetTypeRole:
			if target.Id == roleId || !enterpriseRoleExists(adminId, target.Id) {
				return nil, ErrEnterpriseTeamWorkScopeInvalid
			}
		default:
			return nil, ErrEnterpriseTeamWorkScopeInvalid
		}
		seen[target] = true
		result = append(result, target)
	}
	if len(result) == 0 {
		return nil, ErrEnterpriseTeamWorkScopeInvalid
	}
	return result, nil
}

// filterProjectPermissionKeys keeps only permission keys within the project scope (others, e.g. enterprise member permissions, are dropped).
func filterProjectPermissionKeys(keys []string) []string {
	// Workspace group included: the preset project member role owns workspace:view, and the role config
	// page does not expose the workspace group, so the key must survive permission replacement.
	projectGroups := map[string]bool{
		permission.GroupKeyWorkspace:     true,
		permission.GroupKeyRequirement:   true,
		permission.GroupKeyDefect:        true,
		permission.GroupKeyGantt:         true,
		permission.GroupKeyVersion:       true,
		permission.GroupKeyKnowledgeBase: true,
		permission.GroupKeyDashboard:     true,
		// Issue invitation group must survive the filter; otherwise ticket:copy_link / ticket:manage
		// checked on the role config page would be silently dropped when saving or copying a role.
		permission.GroupKeyTicket:            true,
		permission.GroupKeyProjectInfo:       true,
		permission.GroupKeyProjectMembers:    true,
		permission.GroupKeyProjectRoles:      true,
		permission.GroupKeyRequirementSetup:  true,
		permission.GroupKeyDefectSetup:       true,
		permission.GroupKeyAutomation:        true,
		permission.GroupKeySLA:               true,
		permission.GroupKeyApplicationMarket: true,
		permission.GroupKeyPlanning:          true,
	}
	result := make([]string, 0, len(keys))
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if seen[key] {
			continue
		}
		perm, ok := permission.GetPermission(key)
		if !ok {
			continue
		}
		if !projectGroups[perm.Group] {
			continue
		}
		seen[key] = true
		result = append(result, key)
	}
	return result
}
