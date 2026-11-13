// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package initialize

import (
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/route"
	"goteams/internal/pkg/lib_web"

	"github.com/gin-gonic/gin"
)

// initGin initializes the Gin HTTP server.
func initGin() {
	if !define.IsDev {
		gin.SetMode(gin.ReleaseMode)
	}

	port := define.Config.WebService["port"]

	// Global middleware: CORS, commercial license gate, request logging.
	middleware := []gin.HandlerFunc{
		corsMiddleware(),
		common.OSSURLRequestMiddleware(),
		ossURLSignMiddleware(),
		business.CommercialLicenseMiddleware(),
	}

	define.WebService = lib_web.InitGin(port, route.Route, middleware...)
}

// corsMiddleware handles cross-origin requests (allows all origins in development).
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, lang")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
