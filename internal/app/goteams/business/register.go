package business

import (
	"context"
	"crypto/rand"
	"errors"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_email"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_redis"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/msql"
)

const verifyCodeTTL = 10 * time.Minute

var (
	ErrVerifyCodeInvalid  = errors.New(`verification code is invalid or expired`)
	ErrVerifyCodeFrequent = errors.New(`verification code requested too frequently`)
	ErrUserAlreadyExists  = errors.New(`user already exists`)
)

// RegisterTenantModel 表示租户注册业务数据。
type RegisterTenantModel struct {
	Username   string
	Password   string
	Name       string
	Lang       string
	Id         int
	AdminId    int
	Avatar     string
	RoleId     int
	IsOwner    bool
	FirstLogin string
	TelNumber  string
	Remark     string
	Status     int
	CreatedAt  int
	UpdatedAt  int
	Token      string
}

// RegisterTenant 注册租户管理员并初始化企业角色和体验套餐。
func RegisterTenant(model RegisterTenantModel) (RegisterTenantModel, error) {
	model.Username = validate.NormalizeEmail(model.Username)
	if UserUsernameExist(model.Username) {
		return RegisterTenantModel{}, ErrUserAlreadyExists
	}
	user, err := CreateUser(CreateUserModel{
		Username: model.Username, Password: model.Password, Name: model.Name, EnterpriseName: model.Name,
	})
	if err != nil {
		return RegisterTenantModel{}, err
	}
	if err = SetUserAdminId(user.UserId); err != nil {
		return RegisterTenantModel{}, err
	}
	adminRoleId, _, err := SeedEnterpriseRoles(user.UserId)
	if err != nil {
		return RegisterTenantModel{}, err
	}
	if err = SetUserRoleId(user.UserId, adminRoleId); err != nil {
		return RegisterTenantModel{}, err
	}
	if err = EnsureDefaultPlan(user.UserId); err != nil {
		return RegisterTenantModel{}, err
	}
	tokenData, err := GenerateLoginToken(user.UserId)
	if err != nil {
		return RegisterTenantModel{}, err
	}
	return registerTenantModelFromToken(model, tokenData)
}

func registerTenantModelFromToken(model RegisterTenantModel, tokenData msql.Params) (RegisterTenantModel, error) {
	var err error
	model.Id, err = parseRowInt(tokenData, `id`)
	if err != nil {
		return RegisterTenantModel{}, err
	}
	model.AdminId, err = parseRowInt(tokenData, `admin_id`)
	if err != nil {
		return RegisterTenantModel{}, err
	}
	model.RoleId, err = parseRowInt(tokenData, `role_id`)
	if err != nil {
		return RegisterTenantModel{}, err
	}
	model.Status, err = parseRowInt(tokenData, `status`)
	if err != nil {
		return RegisterTenantModel{}, err
	}
	model.CreatedAt, err = parseRowInt(tokenData, `created_at`)
	if err != nil {
		return RegisterTenantModel{}, err
	}
	model.UpdatedAt, err = parseRowInt(tokenData, `updated_at`)
	if err != nil {
		return RegisterTenantModel{}, err
	}
	model.Password = ``
	model.Username = tokenData[`username`]
	model.Name = tokenData[`name`]
	model.Avatar = tokenData[`avatar`]
	model.IsOwner = tokenData[`is_owner`] == `1`
	model.FirstLogin = tokenData[`first_login`]
	model.TelNumber = tokenData[`tel_number`]
	model.Remark = tokenData[`remark`]
	model.Token = tokenData[`token`]
	return model, nil
}

// findUserByEmailOrUsername resolves the account owning an address: the bound email column first,
// then the legacy accounts whose login account is the address itself.
func findUserByEmailOrUsername(email string) (msql.Params, error) {
	user, err := GetUserByEmail(email)
	if err != nil {
		return nil, err
	}
	if len(user) > 0 {
		return user, nil
	}
	return GetUserByUsername(email)
}

// SendPasswordVerifyCode 发送找回密码验证码。
func SendPasswordVerifyCode(username string, lang string) error {
	username = validate.NormalizeEmail(username)
	user, err := findUserByEmailOrUsername(username)
	if err != nil {
		return err
	}
	if len(user) == 0 {
		return errUserNotExist
	}
	throttleKey := `goteams:password_reset:throttle:` + username
	ok, err := define.Redis.SetNX(context.Background(), throttleKey, `1`, time.Minute).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrVerifyCodeFrequent
	}
	code, err := randomDigits(6)
	if err != nil {
		_ = lib_redis.Del(define.Redis, throttleKey)
		return err
	}
	codeKey := passwordResetCodeKey(username)
	if err = lib_redis.Set(define.Redis, codeKey, code, verifyCodeTTL); err != nil {
		_ = lib_redis.Del(define.Redis, throttleKey)
		return err
	}
	body := lib_i18n.Show(lang, `email_password_reset_body`, code)
	subject := lib_i18n.Show(lang, `email_password_reset_subject`)
	if err = lib_email.SendEmail([]string{username}, subject, body, `text/html`); err != nil {
		_ = lib_redis.Del(define.Redis, codeKey, throttleKey)
		return err
	}
	return nil
}

// ResetPasswordByVerifyCode 使用验证码重置密码。
func ResetPasswordByVerifyCode(username string, verifyCode string, newPassword string) error {
	username = validate.NormalizeEmail(username)
	codeKey := passwordResetCodeKey(username)
	stored, err := lib_redis.Get(define.Redis, codeKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrVerifyCodeInvalid
		}
		return err
	}
	if stored != verifyCode {
		return ErrVerifyCodeInvalid
	}
	user, err := findUserByEmailOrUsername(username)
	if err != nil {
		return err
	}
	if len(user) == 0 {
		return errUserNotExist
	}
	// 找回密码后踢掉该用户所有旧会话（keepSessionToken 为空）
	if err = updateUserPassword(cast.ToInt(user[`admin_id`]), cast.ToInt(user[`id`]), newPassword, ``); err != nil {
		return err
	}
	return lib_redis.Del(define.Redis, codeKey, `goteams:password_reset:throttle:`+username)
}

func passwordResetCodeKey(username string) string {
	return `goteams:password_reset:code:` + username
}

func randomDigits(length int) (string, error) {
	if length <= 0 {
		return ``, errors.New(`invalid verification code length`)
	}
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ``, err
	}
	for i := range b {
		b[i] = '0' + b[i]%10
	}
	return string(b), nil
}

// registerCodeKey 注册验证码 Redis 键。
func registerCodeKey(username string) string {
	return `goteams:register:code:` + username
}

// SendRegisterVerifyCode 发送注册邮箱验证码（注册前校验邮箱归属）。
func SendRegisterVerifyCode(username string, lang string) error {
	username = validate.NormalizeEmail(username)
	if UserUsernameExist(username) {
		return ErrUserAlreadyExists
	}
	if err := validate.ValidateForgotPassword(username); err != nil {
		return err
	}
	throttleKey := `goteams:register:throttle:` + username
	ok, err := define.Redis.SetNX(context.Background(), throttleKey, `1`, time.Minute).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrVerifyCodeFrequent
	}
	code, err := randomDigits(6)
	if err != nil {
		_ = lib_redis.Del(define.Redis, throttleKey)
		return err
	}
	codeKey := registerCodeKey(username)
	if err = lib_redis.Set(define.Redis, codeKey, code, verifyCodeTTL); err != nil {
		_ = lib_redis.Del(define.Redis, throttleKey)
		return err
	}
	body := lib_i18n.Show(lang, `email_register_code_body`, code)
	subject := lib_i18n.Show(lang, `email_register_code_subject`)
	if err = lib_email.SendEmail([]string{username}, subject, body, `text/html`); err != nil {
		_ = lib_redis.Del(define.Redis, codeKey, throttleKey)
		return err
	}
	return nil
}

// VerifyRegisterCode 校验注册邮箱验证码，校验成功后失效（一次性）。
func VerifyRegisterCode(username string, verifyCode string) error {
	username = validate.NormalizeEmail(username)
	codeKey := registerCodeKey(username)
	stored, err := lib_redis.Get(define.Redis, codeKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrVerifyCodeInvalid
		}
		return err
	}
	if stored != verifyCode {
		return ErrVerifyCodeInvalid
	}
	_ = lib_redis.Del(define.Redis, codeKey, `goteams:register:throttle:`+username)
	return nil
}
