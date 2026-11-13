// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// PermissionMiddleware is the permission check middleware (based on permission key).
// Retrieves userId and adminId from context, then calls HasPermission to check.
func PermissionMiddleware(permKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := lib_i18n.GetLang(c)
		userId := cast.ToInt(c.GetInt(`userId`))
		adminId := cast.ToInt(c.GetInt(`adminId`))
		wsId := c.Param(`wsId`)

		if !HasPermission(adminId, userId, wsId, permKey) {
			c.JSON(http.StatusForbidden, define.Response{
				Code:    http.StatusForbidden,
				Message: lib_i18n.Show(lang, define.MsgForbidden),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// PermissionAnyMiddleware checks whether the current user has any of the given permissions.
func PermissionAnyMiddleware(permKeys ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := lib_i18n.GetLang(c)
		userId := cast.ToInt(c.GetInt(`userId`))
		adminId := cast.ToInt(c.GetInt(`adminId`))
		wsId := c.Param(`wsId`)

		for _, permKey := range permKeys {
			if HasPermission(adminId, userId, wsId, permKey) {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, define.Response{
			Code:    http.StatusForbidden,
			Message: lib_i18n.Show(lang, define.MsgForbidden),
		})
		c.Abort()
	}
}
