// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package lib_web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/zhimaAi/go_tools/logs"
)

// WebRun starts the HTTP server (blocking).
func WebRun(ws *http.Server) {
	logs.Info(`WebService Listen on %s`, ws.Addr)
	err := ws.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		logs.Info(`WebService Closed`)
	} else if err != nil {
		logs.Error(err.Error())
		panic(`WebService listen error`)
	}
}

// Shutdown gracefully shuts down the HTTP server (5-second timeout).
func Shutdown(ws *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ws.Shutdown(ctx); err != nil {
		logs.Error(err.Error())
		panic(`WebService shutdown error`)
	}
	logs.Info(`WebService Shutdown`)
}
