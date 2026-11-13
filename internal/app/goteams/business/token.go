// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// ErrLoginWrong indicates a wrong username or password.
var ErrLoginWrong = errors.New(`wrong username or password`)

// ErrUserNotExist indicates the user no longer exists.
var ErrUserNotExist = errors.New(`user not exist`)

// ErrAccountDisabled indicates the account has been disabled by an administrator.
var ErrAccountDisabled = errors.New(`account disabled`)

// errUserNotExist 兼容旧引用（register.go 等），与 ErrUserNotExist 等价。
var errUserNotExist = ErrUserNotExist

// GenerateLoginToken 查询用户并创建标准会话，返回与登录相同格式的用户数据（含 token）。
func GenerateLoginToken(userId int) (msql.Params, error) {
	return GenerateLoginTokenWithTTL(userId, cast.ToInt(define.Config.Session[`ttl`]))
}

// GenerateLoginTokenWithTTL 使用指定初始有效期（秒）创建会话，用于运营短期登录链接等场景。
// 链接在有效期内未被使用则自动失效；一旦使用，认证中间件会按标准滑动窗口续期。
func GenerateLoginTokenWithTTL(userId int, ttl int) (msql.Params, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	user, err := m.Field(userTokenFields + `,remark`).Where(`id`, cast.ToString(userId)).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	if len(user) == 0 {
		return nil, ErrUserNotExist
	}
	token, err := common.CreateSessionWithTTL(common.SessionData{
		UserId:  userId,
		AdminId: cast.ToInt(user[`admin_id`]),
	}, time.Duration(ttl)*time.Second)
	if err != nil {
		return nil, err
	}
	user[`token`] = token
	return user, nil
}

// userTokenFields lists the user columns required to build the login result.
// is_owner must stay in this list: the frontend stores the login response as its user info and
// drives every super-admin branch off it, so a missing column silently demotes the tenant owner.
const userTokenFields = `id,admin_id,username,name,avatar,role_id,is_owner,first_login,tel_number,status,created_at,updated_at`

// UserModel describes the user business data returned together with a token.
type UserModel struct {
	Id         int
	AdminId    int
	Username   string
	Name       string
	Avatar     string
	RoleId     int
	IsOwner    bool
	FirstLogin string
	TelNumber  string
	Status     int
	CreatedAt  int64
	UpdatedAt  int64
}

// LoginModel describes the login use case.
type LoginModel struct {
	Username string
	Password string
	Ip       string
	Ua       string
	User     UserModel
	Token    string
}

// RefreshTokenModel describes the refresh-token use case.
type RefreshTokenModel struct {
	UserId int
	Token  string
	User   UserModel
}

// Login verifies the credentials and issues a server-side session token (stored in Redis).
// It returns ErrLoginWrong when the user does not exist or the password mismatches.
func Login(model LoginModel) (LoginModel, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Field(userTokenFields + `,password`).Where2([][]string{
		{`username`, model.Username},
		{`deleted_at`, `0`},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return LoginModel{}, err
	}
	if len(row) == 0 {
		return LoginModel{}, ErrLoginWrong
	}
	if !common.VerifyPassword(row[`password`], model.Password) {
		return LoginModel{}, ErrLoginWrong
	}
	// Checked only after the password matches, so a wrong guess cannot probe whether an account is disabled.
	if cast.ToInt(row[`status`]) == define.MemberStatusDisabled {
		return LoginModel{}, ErrAccountDisabled
	}

	user, err := toUserModel(row)
	if err != nil {
		return LoginModel{}, err
	}
	token, err := common.CreateSession(common.SessionData{
		UserId:  user.Id,
		AdminId: user.AdminId,
		Ip:      model.Ip,
		Ua:      model.Ua,
	})
	if err != nil {
		return LoginModel{}, err
	}

	model.Password = ``
	model.User = user
	model.Token = token
	return model, nil
}

// RefreshLoginToken returns the logged-in user's info together with the current session token.
// With server-side sessions the token itself no longer rotates: sliding renewal happens in the auth middleware.
func RefreshLoginToken(model RefreshTokenModel) (RefreshTokenModel, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Field(userTokenFields).Where(`id`, cast.ToString(model.UserId)).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return RefreshTokenModel{}, err
	}
	if len(row) == 0 {
		return RefreshTokenModel{}, ErrUserNotExist
	}
	// Defence in depth: disabling a member revokes their sessions, but if that revocation was
	// missed (e.g. Redis hiccup) a surviving session must not be able to renew itself.
	if cast.ToInt(row[`status`]) == define.MemberStatusDisabled {
		return RefreshTokenModel{}, ErrAccountDisabled
	}

	user, err := toUserModel(row)
	if err != nil {
		return RefreshTokenModel{}, err
	}

	model.User = user
	return model, nil
}

// toUserModel converts a user row into the business model.
func toUserModel(row msql.Params) (UserModel, error) {
	id, err := strconv.Atoi(row[`id`])
	if err != nil {
		return UserModel{}, fmt.Errorf(`parse user id: %w`, err)
	}
	adminId, err := strconv.Atoi(row[`admin_id`])
	if err != nil {
		return UserModel{}, fmt.Errorf(`parse user admin_id: %w`, err)
	}
	roleId, err := strconv.Atoi(row[`role_id`])
	if err != nil {
		return UserModel{}, fmt.Errorf(`parse user role_id: %w`, err)
	}
	status, err := strconv.Atoi(row[`status`])
	if err != nil {
		return UserModel{}, fmt.Errorf(`parse user status: %w`, err)
	}
	createdAt, err := strconv.ParseInt(row[`created_at`], 10, 64)
	if err != nil {
		return UserModel{}, fmt.Errorf(`parse user created_at: %w`, err)
	}
	updatedAt, err := strconv.ParseInt(row[`updated_at`], 10, 64)
	if err != nil {
		return UserModel{}, fmt.Errorf(`parse user updated_at: %w`, err)
	}

	return UserModel{
		Id:         id,
		AdminId:    adminId,
		Username:   row[`username`],
		Name:       row[`name`],
		Avatar:     row[`avatar`],
		RoleId:     roleId,
		IsOwner:    row[`is_owner`] == `1`,
		FirstLogin: row[`first_login`],
		TelNumber:  row[`tel_number`],
		Status:     status,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}, nil
}
