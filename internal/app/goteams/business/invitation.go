// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ErrInvitationInvalid indicates the invitation link is missing, expired, stopped or full.
var ErrInvitationInvalid = errors.New("invitation invalid")

// ErrInvitePhoneRequired indicates the invitation requires a phone number but none was provided.
var ErrInvitePhoneRequired = errors.New("invite phone required")

// ErrInviteEmailRequired indicates the invitation requires an email but none was provided.
var ErrInviteEmailRequired = errors.New("invite email required")

// InvitationModel describes member invitation business data.
type InvitationModel struct {
	Id             int
	AdminId        int
	CreatedBy      int
	Token          string
	RoleId         int
	RoleName       string
	InviterName    string
	EnterpriseName string
	InviteLimit    int
	AcceptedCount  int
	PhoneRequired  bool
	EmailRequired  bool
	DepartmentIds  []int
	ExpiredAt      int64
}

// CreateInvitationModel describes the create-invitation use case.
type CreateInvitationModel struct {
	AdminId       int
	CreatedBy     int
	RoleId        int
	InviteLimit   int
	ExpireMinutes int
	PhoneRequired bool
	EmailRequired bool
	DepartmentIds []int
	Id            int
	Token         string
	ExpiredAt     int64
}

// CreateInvitation stops any active invitation of the tenant and creates a new one.
func CreateInvitation(model CreateInvitationModel) (CreateInvitationModel, error) {
	if !enterpriseRoleExists(model.AdminId, model.RoleId) {
		return CreateInvitationModel{}, ErrEnterpriseRoleNotFound
	}
	var err error
	model.DepartmentIds, err = ValidateTenantDepartmentIds(model.AdminId, model.DepartmentIds)
	if err != nil {
		return CreateInvitationModel{}, err
	}
	if err := StopInvitation(model.AdminId); err != nil {
		return CreateInvitationModel{}, err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		logs.Error("generate invitation token failed: %s", err.Error())
		return CreateInvitationModel{}, err
	}
	model.Token = hex.EncodeToString(buf)
	now := tool.Time2Int()
	model.ExpiredAt = int64(now) + int64(model.ExpireMinutes)*60
	m := msql.Model(define.TableMemberInvitations, define.Postgres)
	id, err := m.Insert(msql.Datas{
		`admin_id`:       model.AdminId,
		`token`:          model.Token,
		`role_id`:        model.RoleId,
		`invite_limit`:   model.InviteLimit,
		`accepted_count`: 0,
		`phone_required`: boolToSmallint(model.PhoneRequired),
		`email_required`: boolToSmallint(model.EmailRequired),
		`department_ids`: intsToCSV(model.DepartmentIds),
		`expired_at`:     model.ExpiredAt,
		`status`:         1,
		`created_by`:     model.CreatedBy,
		`created_at`:     now,
		`updated_at`:     now,
	}, `id`)
	if err != nil {
		logs.Error("create invitation error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return CreateInvitationModel{}, err
	}
	model.Id = cast.ToInt(id)
	return model, nil
}

// GetActiveInvitation returns the tenant's current usable invitation; found=false when none exists.
func GetActiveInvitation(adminId int) (InvitationModel, bool, error) {
	m := msql.Model(define.TableMemberInvitations, define.Postgres)
	row, err := m.Field(`id, admin_id, created_by, token, role_id, invite_limit, accepted_count, phone_required, email_required, department_ids, expired_at`).
		Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`status`, `1`},
		}).Order(`id DESC`).Find()
	if err != nil {
		logs.Error("get active invitation error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return InvitationModel{}, false, err
	}
	if len(row) == 0 {
		return InvitationModel{}, false, nil
	}
	model, err := toInvitationModel(row)
	if err != nil {
		return InvitationModel{}, false, err
	}
	if !invitationUsable(model) {
		return InvitationModel{}, false, nil
	}
	return model, true, nil
}

// StopInvitation stops all active invitations of the tenant.
func StopInvitation(adminId int) error {
	m := msql.Model(define.TableMemberInvitations, define.Postgres)
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`status`, `1`},
	}).Update(msql.Datas{`status`: 0, `updated_at`: tool.Time2Int()})
	if err != nil {
		logs.Error("stop invitation error: sql=%s, err=%s", m.GetLastSql(), err.Error())
	}
	return err
}

// GetInvitationByToken returns a usable invitation by token (with role name), or ErrInvitationInvalid.
func GetInvitationByToken(token string) (InvitationModel, error) {
	if token == `` {
		return InvitationModel{}, ErrInvitationInvalid
	}
	m := msql.Model(define.TableMemberInvitations, define.Postgres)
	row, err := m.Field(`id, admin_id, created_by, token, role_id, invite_limit, accepted_count, phone_required, email_required, department_ids, expired_at`).
		Where(`token`, token).Where(`status`, `1`).Find()
	if err != nil {
		logs.Error("get invitation by token error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return InvitationModel{}, err
	}
	if len(row) == 0 {
		return InvitationModel{}, ErrInvitationInvalid
	}
	model, err := toInvitationModel(row)
	if err != nil {
		return InvitationModel{}, err
	}
	if !invitationUsable(model) {
		return InvitationModel{}, ErrInvitationInvalid
	}
	roleM := msql.Model(define.TableRoles, define.Postgres)
	roleName, err := roleM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`id`, cast.ToString(model.RoleId)},
	}).Value(`name`)
	if err != nil {
		logs.Error("get invitation role name error: sql=%s, err=%s", roleM.GetLastSql(), err.Error())
		return InvitationModel{}, err
	}
	model.RoleName = roleName
	return model, nil
}

// GetInvitationInfoByToken returns the public invitation page details.
func GetInvitationInfoByToken(token string) (InvitationModel, error) {
	model, err := GetInvitationByToken(token)
	if err != nil {
		return InvitationModel{}, err
	}
	inviter, err := GetUserInfo(model.AdminId, model.CreatedBy)
	if err != nil {
		return InvitationModel{}, err
	}
	if len(inviter) == 0 {
		return InvitationModel{}, ErrInvitationInvalid
	}
	enterpriseName, err := GetEnterpriseName(model.AdminId)
	if err != nil {
		return InvitationModel{}, err
	}
	model.InviterName = inviter[`name`]
	model.EnterpriseName = enterpriseName
	return model, nil
}

// AcceptInvitationModel describes the accept-invitation use case.
type AcceptInvitationModel struct {
	Token      string
	Username   string
	Password   string
	Name       string
	Phone      string
	Email      string
	VerifyCode string
	Avatar     string
	UserId     int
}

// AcceptInvitation atomically consumes one invitation slot and creates the tenant user.
func AcceptInvitation(model AcceptInvitationModel) (AcceptInvitationModel, error) {
	inv, err := GetInvitationByToken(model.Token)
	if err != nil {
		return AcceptInvitationModel{}, err
	}
	model.Username = validate.NormalizeEmail(model.Username)
	model.Name = strings.TrimSpace(model.Name)
	model.Phone = strings.TrimSpace(model.Phone)
	model.Email = validate.NormalizeEmail(model.Email)
	// Whether the phone number is required is decided by the invitation settings
	if inv.PhoneRequired && model.Phone == `` {
		return AcceptInvitationModel{}, ErrInvitePhoneRequired
	}
	if inv.EmailRequired && model.Email == `` {
		return AcceptInvitationModel{}, ErrInviteEmailRequired
	}
	if model.Email != `` {
		if err := validate.ValidateMemberEmail(model.Email); err != nil {
			return AcceptInvitationModel{}, err
		}
		// Availability is checked before the code is consumed, so a rejected email keeps its code.
		emailExists, emailErr := UserEmailExist(model.Email, 0)
		if emailErr != nil {
			return AcceptInvitationModel{}, emailErr
		}
		if emailExists {
			return AcceptInvitationModel{}, ErrUserEmailExists
		}
		// An email is only accepted once its code verified ownership; the invitation slot is
		// consumed only afterwards.
		if err := VerifyInviteEmailCode(model.Email, model.VerifyCode); err != nil {
			return AcceptInvitationModel{}, err
		}
	}
	exists, err := enterpriseUsernameExists(model.Username, 0)
	if err != nil {
		return AcceptInvitationModel{}, err
	}
	if exists {
		return AcceptInvitationModel{}, ErrUserUsernameExists
	}
	// 套餐席位需要在事务内严格校验；先确保租户套餐存在
	if err = EnsureDefaultPlan(inv.AdminId); err != nil {
		return AcceptInvitationModel{}, err
	}

	now := tool.Time2Int()
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return AcceptInvitationModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	// 席位校验：席位不足对被邀请人统一归入"邀请已失效"
	if err = CheckMemberQuotaTx(tx, inv.AdminId); err != nil {
		var quotaErr *QuotaExceededError
		if errors.As(err, &quotaErr) {
			return AcceptInvitationModel{}, ErrInvitationInvalid
		}
		return AcceptInvitationModel{}, err
	}
	// 原子占用一个邀请名额，防止并发超卖
	rows, err := msql.RawValues(define.Postgres, `UPDATE member_invitations
		SET accepted_count = accepted_count + 1, updated_at = $1
		WHERE id = $2 AND status = 1 AND expired_at > $1 AND accepted_count < invite_limit
		RETURNING id`, tx, now, inv.Id)
	if err != nil {
		logs.Error("consume invitation slot error: %s", err.Error())
		return AcceptInvitationModel{}, err
	}
	if len(rows) == 0 {
		return AcceptInvitationModel{}, ErrInvitationInvalid
	}
	userId, err := createUserTx(tx, CreateUserModel{
		Username: model.Username, Password: model.Password, Name: model.Name, Phone: model.Phone,
		Email: model.Email, AdminId: inv.AdminId, RoleId: inv.RoleId,
	}, inv.Id)
	if err != nil {
		if isUserUsernameConflict(err) {
			return AcceptInvitationModel{}, ErrUserUsernameExists
		}
		if isUserEmailConflict(err) {
			return AcceptInvitationModel{}, ErrUserEmailExists
		}
		return AcceptInvitationModel{}, err
	}
	// Departments may have been deleted while the invitation was active: silently drop missing ones.
	if deptIds := FilterExistingDepartmentIds(inv.AdminId, inv.DepartmentIds); len(deptIds) > 0 {
		if err = setUserDepartmentsTx(tx, inv.AdminId, userId, deptIds); err != nil {
			return AcceptInvitationModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return AcceptInvitationModel{}, err
	}

	if model.Avatar != `` {
		// ponytail: on avatar claim failure, remove the user but do not roll back the count; the deviation only affects the invitation quota
		if _, err = ClaimPendingAvatar(inv.AdminId, userId, model.Avatar); err != nil {
			if removeErr := purgeEnterpriseMember(inv.AdminId, userId); removeErr != nil {
				logs.Error("rollback invited member error: %s", removeErr.Error())
			}
			return AcceptInvitationModel{}, err
		}
	}
	model.UserId = userId
	// Self-service join: the member is their own operator, so the log shows who joined via the link.
	writeMemberLog(inv.AdminId, userId, userId, define.MemberLogActionCreate)
	return model, nil
}

func invitationUsable(model InvitationModel) bool {
	return model.ExpiredAt > int64(tool.Time2Int()) && model.AcceptedCount < model.InviteLimit
}

// boolToSmallint converts a bool to 1/0 for SMALLINT storage.
func boolToSmallint(b bool) int {
	if b {
		return 1
	}
	return 0
}

func toInvitationModel(row msql.Params) (InvitationModel, error) {
	id, err := strconv.Atoi(row[`id`])
	if err != nil {
		return InvitationModel{}, fmt.Errorf("parse invitation id: %w", err)
	}
	adminId, err := strconv.Atoi(row[`admin_id`])
	if err != nil {
		return InvitationModel{}, fmt.Errorf("parse invitation admin id: %w", err)
	}
	createdBy, err := strconv.Atoi(row[`created_by`])
	if err != nil {
		return InvitationModel{}, fmt.Errorf("parse invitation creator id: %w", err)
	}
	roleId, err := strconv.Atoi(row[`role_id`])
	if err != nil {
		return InvitationModel{}, fmt.Errorf("parse invitation role id: %w", err)
	}
	limit, err := strconv.Atoi(row[`invite_limit`])
	if err != nil {
		return InvitationModel{}, fmt.Errorf("parse invitation limit: %w", err)
	}
	accepted, err := strconv.Atoi(row[`accepted_count`])
	if err != nil {
		return InvitationModel{}, fmt.Errorf("parse invitation accepted count: %w", err)
	}
	expiredAt, err := strconv.ParseInt(row[`expired_at`], 10, 64)
	if err != nil {
		return InvitationModel{}, fmt.Errorf("parse invitation expired at: %w", err)
	}
	return InvitationModel{
		Id: id, AdminId: adminId, CreatedBy: createdBy, Token: row[`token`], RoleId: roleId,
		InviteLimit: limit, AcceptedCount: accepted, PhoneRequired: cast.ToInt(row[`phone_required`]) > 0,
		EmailRequired: cast.ToInt(row[`email_required`]) > 0,
		DepartmentIds: parseCSVInts(row[`department_ids`]), ExpiredAt: expiredAt,
	}, nil
}

// intsToCSV joins ids with commas for VARCHAR storage; empty slice becomes an empty string.
func intsToCSV(ids []int) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, `,`)
}

// parseCSVInts parses a comma-separated id string, silently dropping invalid segments.
func parseCSVInts(value string) []int {
	if strings.TrimSpace(value) == `` {
		return nil
	}
	parts := strings.Split(value, `,`)
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		if id, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}
