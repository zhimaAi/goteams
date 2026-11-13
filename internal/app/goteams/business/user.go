// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const maxAvatarSize int64 = 2 * 1024 * 1024

var (
	ErrAvatarInvalid   = errors.New(`invalid avatar image`)
	ErrAvatarTooLarge  = errors.New(`avatar too large`)
	ErrAvatarNotFound  = errors.New(`avatar not found`)
	ErrUserEmailExists = errors.New(`user email exists`)
)

// GetUserInfo returns user info by ID (requires admin_id isolation).
func GetUserInfo(adminId int, userId int) (msql.Params, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	return m.Field(`id, admin_id, username, password, name, avatar, role_id, first_login, tel_number, email, remark, status, created_at, updated_at, is_owner`).
		Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`id`, cast.ToString(userId)},
		}).Find()
}

// GetUserByUsername returns a user by username (for login, no admin_id restriction).
func GetUserByUsername(username string) (msql.Params, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	return m.Where(`username`, strings.ToLower(strings.TrimSpace(username))).Find()
}

// UserUsernameExist checks whether a username is already registered (global uniqueness check).
func UserUsernameExist(username string) bool {
	m := msql.Model(define.TableUsers, define.Postgres)
	id, err := m.Where(`username`, strings.ToLower(strings.TrimSpace(username))).Value(`id`)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return false
	}
	return len(id) > 0
}

// GetUserByEmail returns the user bound to the given email; an empty email matches nobody because
// every unbound user stores an empty value and the unique index skips it.
func GetUserByEmail(email string) (msql.Params, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) == 0 {
		return msql.Params{}, nil
	}
	m := msql.Model(define.TableUsers, define.Postgres)
	return m.Where(`email`, email).Find()
}

// UserEmailExist checks whether the email is already bound by another user (global uniqueness).
// An empty email is never taken, so several unbound members can coexist.
func UserEmailExist(email string, excludedUserId int) (bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) == 0 {
		return false, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableUsers+`
		WHERE email=$1 AND ($2=0 OR id<>$2) LIMIT 1`, nil, email, excludedUserId)
	if err != nil {
		logs.Error("check user email exists error: %s", err.Error())
		return false, err
	}
	return len(rows) > 0, nil
}

// CreateUserModel describes user creation data.
type CreateUserModel struct {
	Username       string
	Password       string
	Name           string
	EnterpriseName string
	Phone          string
	Email          string
	Remark         string
	AdminId        int
	RoleId         int
	UserId         int
}

// CreateUser creates a new user and returns the resulting model.
func CreateUser(model CreateUserModel) (CreateUserModel, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	model.Username = strings.ToLower(strings.TrimSpace(model.Username))
	if len(model.Name) == 0 {
		model.Name = model.Username
	}
	// The account is globally unique; do an explicit check first for a friendly message (concurrency is covered by the unique index).
	if UserUsernameExist(model.Username) {
		return CreateUserModel{}, ErrUserUsernameExists
	}
	// New tenants no longer default the enterprise name to the admin's name: keep it empty so the
	// frontend shows the "unnamed enterprise" placeholder until the root admin renames it.

	hashed, err := common.HashPassword(model.Password)
	if err != nil {
		logs.Error("hash password failed: %s", err.Error())
		return CreateUserModel{}, err
	}

	data := msql.Datas{
		`username`:        model.Username,
		`password`:        hashed,
		`name`:            model.Name,
		`enterprise_name`: model.EnterpriseName,
		`tel_number`:      model.Phone,
		`email`:           model.Email,
		`remark`:          model.Remark,
		`role_id`:         model.RoleId,
		`admin_id`:        model.AdminId,
		`created_at`:      tool.Time2Int(),
		`updated_at`:      tool.Time2Int(),
	}
	id, err := m.Insert(data, `id`)
	if err != nil {
		// The account is globally unique; on unique-index conflict return a business error for the upper layer to surface.
		if isUserUsernameConflict(err) {
			return CreateUserModel{}, ErrUserUsernameExists
		}
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return CreateUserModel{}, err
	}
	model.UserId = cast.ToInt(id)
	return model, nil
}

// createUserTx creates a tenant user inside the given transaction and returns the user ID.
// Member quota must be validated in the same transaction (CheckMemberQuotaTx) beforehand.
func createUserTx(tx *sql.Tx, model CreateUserModel, invitationId int) (int, error) {
	hashed, err := common.HashPassword(model.Password)
	if err != nil {
		logs.Error("hash password failed: %s", err.Error())
		return 0, err
	}
	now := tool.Time2Int()
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO users
		(username, password, name, tel_number, email, remark, role_id, admin_id, invitation_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10) RETURNING id`, tx,
		strings.ToLower(strings.TrimSpace(model.Username)), hashed, model.Name, model.Phone,
		strings.ToLower(strings.TrimSpace(model.Email)), model.Remark,
		model.RoleId, model.AdminId, invitationId, now)
	if err != nil {
		logs.Error("create user in tx failed: %s", err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 0, errors.New("user insert returned no row")
	}
	id, err := strconv.Atoi(rows[0][`id`])
	if err != nil {
		return 0, fmt.Errorf("parse user id: %w", err)
	}
	return id, nil
}

// SetUserAdminId sets the user's admin_id to their own ID and marks them as the tenant owner
// (called during registration/initialization). admin_id stays fixed afterwards even if ownership
// is later transferred, so it keeps working as the tenant isolation key.
func SetUserAdminId(userId int) error {
	m := msql.Model(define.TableUsers, define.Postgres)
	now := tool.Time2Int()
	data := msql.Datas{
		`admin_id`:   userId,
		`is_owner`:   1,
		`updated_at`: now,
	}
	_, err := m.Where(`id`, cast.ToString(userId)).Update(data)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
	}
	return err
}

// UpdateUserProfile updates the user's profile information (requires admin_id isolation).
func UpdateUserProfile(adminId int, userId int, name string) error {
	m := msql.Model(define.TableUsers, define.Postgres)
	now := tool.Time2Int()
	data := msql.Datas{
		`name`:       name,
		`updated_at`: now,
	}
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(userId)},
	}).Update(data)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
	}
	return err
}

// UpdateUserEmail binds, rebinds (or, with an empty value, unbinds) the user's email.
func UpdateUserEmail(adminId int, userId int, email string) error {
	m := msql.Model(define.TableUsers, define.Postgres)
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(userId)},
	}).Update(msql.Datas{`email`: strings.ToLower(strings.TrimSpace(email)), `updated_at`: tool.Time2Int()})
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
	}
	return err
}

// SetUserRoleId sets the user's enterprise role_id (used after enterprise roles are seeded).
func SetUserRoleId(userId int, roleId int) error {
	m := msql.Model(define.TableUsers, define.Postgres)
	now := tool.Time2Int()
	data := msql.Datas{
		`role_id`:    roleId,
		`updated_at`: now,
	}
	_, err := m.Where(`id`, cast.ToString(userId)).Update(data)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
	}
	return err
}

// ErrPasswordWrong indicates the old password does not match.
var ErrPasswordWrong = errors.New(`old password wrong`)

// ChangeOwnPasswordModel describes the change-own-password use case.
type ChangeOwnPasswordModel struct {
	AdminId      int
	UserId       int
	OldPassword  string
	NewPassword  string
	SessionToken string // current session, kept alive after the change
}

// ChangeOwnPassword verifies the old password, updates it and kills the user's other sessions.
func ChangeOwnPassword(model ChangeOwnPasswordModel) error {
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Field(`id,password`).Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`id`, cast.ToString(model.UserId)},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	if len(row) == 0 {
		return ErrUserNotExist
	}
	if !common.VerifyPassword(row[`password`], model.OldPassword) {
		return ErrPasswordWrong
	}
	return updateUserPassword(model.AdminId, model.UserId, model.NewPassword, model.SessionToken)
}

// updateUserPassword updates the user's password (requires admin_id isolation).
// newPassword is stored as a bcrypt hash.
// After a successful update all sessions of the user are removed except keepSessionToken (empty = remove all).
func updateUserPassword(adminId int, userId int, newPassword string, keepSessionToken string) error {
	m := msql.Model(define.TableUsers, define.Postgres)
	now := tool.Time2Int()

	hashed, err := common.HashPassword(newPassword)
	if err != nil {
		logs.Error("hash password failed: %s", err.Error())
		return err
	}

	data := msql.Datas{
		`password`:   hashed,
		`updated_at`: now,
	}
	_, err = m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(userId)},
	}).Update(data)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}

	// The password changed, so old sessions should no longer be valid; on delete failure just log it, don't roll back the business op.
	common.DeleteUserSessions(userId, keepSessionToken)
	return nil
}

// SaveUserAvatar validates and saves the current user's avatar.
func SaveUserAvatar(adminId int, userId int, file *multipart.FileHeader) (string, error) {
	if file == nil || file.Size <= 0 {
		return ``, ErrAvatarInvalid
	}
	if file.Size > maxAvatarSize {
		return ``, ErrAvatarTooLarge
	}

	opened, err := file.Open()
	if err != nil {
		return ``, err
	}
	defer func() { _ = opened.Close() }()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(opened, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return ``, readErr
	}
	contentType := http.DetectContentType(header[:n])
	ext := ``
	switch contentType {
	case `image/jpeg`:
		ext = `.jpg`
	case `image/png`:
		ext = `.png`
	default:
		return ``, ErrAvatarInvalid
	}
	if _, err = opened.Seek(0, io.SeekStart); err != nil {
		return ``, err
	}

	user, err := GetUserInfo(adminId, userId)
	if err != nil {
		return ``, err
	}
	if len(user) == 0 {
		return ``, ErrAvatarNotFound
	}

	storedPath, err := lib_storage.M.Upload(adminId, lib_storage.BusinessEnterprise, ext, opened)
	if err != nil {
		return ``, err
	}
	oldAvatar := user[`avatar`]
	if err = UpdateUserAvatar(adminId, userId, storedPath); err != nil {
		_ = lib_storage.M.Delete(storedPath)
		return ``, err
	}
	removeOldAvatarFile(adminId, userId, oldAvatar, storedPath)
	return storedPath, nil
}

// SavePendingAvatar validates and temporarily stores the avatar for a member to be created, returning the path to submit to the create-member endpoint.
func SavePendingAvatar(adminId int, file *multipart.FileHeader) (string, error) {
	if file == nil || file.Size <= 0 {
		return ``, ErrAvatarInvalid
	}
	if file.Size > maxAvatarSize {
		return ``, ErrAvatarTooLarge
	}

	opened, err := file.Open()
	if err != nil {
		return ``, err
	}
	defer func() { _ = opened.Close() }()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(opened, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return ``, readErr
	}
	ext := ``
	switch http.DetectContentType(header[:n]) {
	case `image/jpeg`:
		ext = `.jpg`
	case `image/png`:
		ext = `.png`
	default:
		return ``, ErrAvatarInvalid
	}
	if _, err = opened.Seek(0, io.SeekStart); err != nil {
		return ``, err
	}

	storedPath, err := lib_storage.M.Upload(adminId, lib_storage.BusinessEnterprise, ext, opened)
	if err != nil {
		return ``, err
	}
	return storedPath, nil
}

// ClaimPendingAvatar validates the tenant ownership of an avatar (uploaded via SavePendingAvatar) and associates it with the new member, returning the final stored_path.
// Avatars are always stored in the avatar namespace; claiming only validates ownership and existence, no migration needed.
func ClaimPendingAvatar(adminId int, userId int, avatar string) (string, error) {
	if !lib_storage.M.IsOwned(adminId, lib_storage.BusinessEnterprise, avatar) {
		return ``, ErrAvatarInvalid
	}
	if !lib_storage.M.Exists(avatar) {
		return ``, ErrAvatarInvalid
	}
	if err := UpdateUserAvatar(adminId, userId, avatar); err != nil {
		return ``, err
	}
	return avatar, nil
}

// UpdateUserAvatar updates the current user's avatar path.
func UpdateUserAvatar(adminId int, userId int, avatar string) error {
	m := msql.Model(define.TableUsers, define.Postgres)
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(userId)},
	}).Update(msql.Datas{`avatar`: avatar, `updated_at`: tool.Time2Int()})
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
	}
	return err
}

// GetUserAvatarPath returns the current user's avatar stored_path (for Serve).
func GetUserAvatarPath(adminId int, userId int) (string, error) {
	user, err := GetUserInfo(adminId, userId)
	if err != nil {
		return ``, err
	}
	if len(user) == 0 || user[`avatar`] == `` {
		return ``, ErrAvatarNotFound
	}
	avatar := user[`avatar`]
	if !lib_storage.M.IsOwned(adminId, lib_storage.BusinessEnterprise, avatar) || !lib_storage.M.Exists(avatar) {
		return ``, ErrAvatarNotFound
	}
	return avatar, nil
}

func removeOldAvatarFile(adminId int, userId int, oldAvatar string, currentAvatar string) {
	if oldAvatar == `` || oldAvatar == currentAvatar {
		return
	}
	if err := lib_storage.M.Delete(oldAvatar); err != nil {
		logs.Error("remove old avatar failed, err:%s", err.Error())
	}
}
