// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package common

import (
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware is the session authentication middleware.
// It does not depend on any business layer; purely validates the Redis session and injects the identity.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := lib_i18n.GetLang(c)

		authHeader := c.GetHeader("Authorization")
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if len(tokenString) == 0 {
			c.JSON(http.StatusUnauthorized, define.Response{
				Code:    http.StatusUnauthorized,
				Message: lib_i18n.Show(lang, define.MsgTokenInvalid),
			})
			c.Abort()
			return
		}

		session, err := GetSession(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, define.Response{
				Code:    http.StatusUnauthorized,
				Message: lib_i18n.Show(lang, define.MsgTokenExpired),
			})
			c.Abort()
			return
		}

		c.Set(`userId`, session.UserId)
		c.Set(`adminId`, session.AdminId)
		c.Set(`sessionToken`, tokenString)
		c.Next()
	}
}
