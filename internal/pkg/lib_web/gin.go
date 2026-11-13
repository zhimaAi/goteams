// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package lib_web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zhimaAi/go_tools/logs"
)

// Route constants.
const NoRoute = `NoRoute`
const NoMethod = `NoMethod`

// Action is a type alias for controller functions.
type Action = func(c *gin.Context)

// Route is a route table: map[HTTP method][path]Action.
type Route map[string]map[string]Action

// InitGin initializes the Gin engine and returns an http.Server.
//   - port: listening port
//   - route: route table
//   - middleware: global middlewares (executed in order)
func InitGin(port string, route Route, middleware ...gin.HandlerFunc) *http.Server {
	engine := gin.Default()

	// Only private-network proxies (edge nginx / container nginx) may supply client IP headers,
	// otherwise X-Forwarded-For sent by arbitrary clients would be trusted for ClientIP().
	// Failure only degrades client IP accuracy and must never block startup or login.
	if err := engine.SetTrustedProxies([]string{`127.0.0.0/8`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`}); err != nil {
		logs.Error(`set trusted proxies error: %s`, err.Error())
	}

	// Register global middlewares.
	if len(middleware) > 0 {
		engine.Use(middleware...)
	}

	// Register routes.
	for method, routers := range route {
		for path, action := range routers {
			switch method {
			case http.MethodGet, http.MethodPost, http.MethodPut,
				http.MethodPatch, http.MethodHead, http.MethodOptions,
				http.MethodDelete, http.MethodConnect, http.MethodTrace:
				engine.Handle(method, path, action)
			case NoRoute:
				engine.NoRoute(action)
			case NoMethod:
				fallthrough
			default:
				engine.HandleMethodNotAllowed = true
				engine.NoMethod(action)
			}
		}
	}

	return &http.Server{Addr: `:` + port, Handler: engine}
}
