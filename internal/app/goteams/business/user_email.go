package business

import (
	"context"
	"errors"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_email"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_redis"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cast"
)

// Email verification code namespaces. Binding is scoped by user id so one account cannot be used
// to flood arbitrary mailboxes; the invite-join flow has no user yet and is scoped by email.
const (
	bindEmailCodePrefix   = `goteams:bind_email:`
	inviteEmailCodePrefix = `goteams:invite_email:`
)

// SendBindEmailCode mails a verification code to the address the current user wants to bind.
func SendBindEmailCode(userId int, email string, lang string) error {
	email = validate.NormalizeEmail(email)
	if err := validate.ValidateMemberEmail(email); err != nil {
		return err
	}
	exists, err := UserEmailExist(email, userId)
	if err != nil {
		return err
	}
	if exists {
		return ErrUserEmailExists
	}
	return sendEmailVerifyCode(bindEmailCodePrefix, cast.ToString(userId), email, lang)
}

// BindUserEmail binds or rebinds the user's email once the verification code passes.
func BindUserEmail(adminId int, userId int, email string, verifyCode string) error {
	email = validate.NormalizeEmail(email)
	if err := validate.ValidateMemberEmail(email); err != nil {
		return err
	}
	exists, err := UserEmailExist(email, userId)
	if err != nil {
		return err
	}
	if exists {
		return ErrUserEmailExists
	}
	if err = consumeEmailVerifyCode(bindEmailCodePrefix, cast.ToString(userId), verifyCode); err != nil {
		return err
	}
	return UpdateUserEmail(adminId, userId, email)
}

// SendInviteEmailCode mails a verification code to the address an invitee wants to submit.
func SendInviteEmailCode(email string, lang string) error {
	email = validate.NormalizeEmail(email)
	if err := validate.ValidateMemberEmail(email); err != nil {
		return err
	}
	exists, err := UserEmailExist(email, 0)
	if err != nil {
		return err
	}
	if exists {
		return ErrUserEmailExists
	}
	return sendEmailVerifyCode(inviteEmailCodePrefix, email, email, lang)
}

// VerifyInviteEmailCode checks the invitee's email code; the code is single-use.
func VerifyInviteEmailCode(email string, verifyCode string) error {
	return consumeEmailVerifyCode(inviteEmailCodePrefix, validate.NormalizeEmail(email), verifyCode)
}

// sendEmailVerifyCode stores a one-time code under prefix+scope and mails it to email.
func sendEmailVerifyCode(prefix string, scope string, email string, lang string) error {
	throttleKey := prefix + `throttle:` + scope
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
	codeKey := prefix + `code:` + scope
	if err = lib_redis.Set(define.Redis, codeKey, code, verifyCodeTTL); err != nil {
		_ = lib_redis.Del(define.Redis, throttleKey)
		return err
	}
	body := lib_i18n.Show(lang, `email_bind_code_body`, code)
	subject := lib_i18n.Show(lang, `email_bind_code_subject`)
	if err = lib_email.SendEmail([]string{email}, subject, body, `text/html`); err != nil {
		_ = lib_redis.Del(define.Redis, codeKey, throttleKey)
		return err
	}
	return nil
}

// consumeEmailVerifyCode checks the stored code and invalidates it on success.
func consumeEmailVerifyCode(prefix string, scope string, verifyCode string) error {
	codeKey := prefix + `code:` + scope
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
	return lib_redis.Del(define.Redis, codeKey, prefix+`throttle:`+scope)
}
