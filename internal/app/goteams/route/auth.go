// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegAuthRoutes registers authentication routes (public, no login required).
func RegAuthRoutes(r lib_web.Route) {
	r[http.MethodPost][`/api/auth/login`] = controller.Login
	r[http.MethodPost][`/api/auth/register`] = controller.Register
	r[http.MethodPost][`/api/auth/send-register-code`] = controller.SendRegisterVerifyCode
	r[http.MethodPost][`/api/auth/verifycode`] = controller.SendVerifyCode
	r[http.MethodPost][`/api/auth/send-verify-code`] = controller.SendVerifyCode
	r[http.MethodGet][`/api/auth/captcha`] = controller.GetCaptcha
	r[http.MethodPost][`/api/auth/checkCaptcha`] = controller.CheckCaptcha
	r[http.MethodPost][`/api/auth/forgot-password`] = controller.ForgotPassword
	r[http.MethodPost][`/api/auth/reset-password`] = controller.ResetPassword
}
