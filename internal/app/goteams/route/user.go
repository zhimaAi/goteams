// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegUserRoutes registers user routes (login required).
func RegUserRoutes(r lib_web.Route) {
	auth := withAuth()

	r[http.MethodGet][`/api/user/profile`] = wrapAction(auth, controller.GetProfile)
	r[http.MethodPut][`/api/user/profile`] = wrapAction(auth, controller.UpdateProfile)
	r[http.MethodGet][`/api/user/avatar`] = wrapAction(auth, controller.GetAvatar)
	r[http.MethodPost][`/api/user/avatar`] = wrapAction(auth, controller.UploadAvatar)
	r[http.MethodPut][`/api/user/password`] = wrapAction(auth, controller.ChangePassword)
	r[http.MethodPost][`/api/user/email/send-code`] = wrapAction(auth, controller.SendBindEmailCode)
	r[http.MethodPut][`/api/user/email`] = wrapAction(auth, controller.BindEmail)
	r[http.MethodGet][`/api/user/refreshToken`] = wrapAction(auth, controller.RefreshToken)
	r[http.MethodPost][`/api/user/logout`] = wrapAction(auth, controller.Logout)
	r[http.MethodGet][`/api/user/permissions`] = wrapAction(auth, controller.GetMyPermissions)
	r[http.MethodGet][`/api/user/contact-preferences`] = wrapAction(auth, controller.GetContactPreferences)
	r[http.MethodPut][`/api/user/contact-preferences/:contactUserId/favorite`] = wrapAction(auth, controller.UpdateFavoriteContact)
	r[http.MethodPut][`/api/user/contact-preferences/recent`] = wrapAction(auth, controller.RecordRecentContacts)
}
