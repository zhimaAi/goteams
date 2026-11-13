// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/initialize"
	"goteams/internal/pkg/lib_web"

	"github.com/zhimaAi/go_tools/logs"
)

func main() {
	// -version prints build identity and exits (used by goteamsctl package verification).
	if len(os.Args) > 1 && (os.Args[1] == `-version` || os.Args[1] == `--version`) {
		fmt.Printf("goteams %s edition=%s commit=%s\n", define.BuildVersion, define.BuildEdition, define.BuildCommit)
		return
	}

	// Set logs directory
	logs.SetLogsDir(define.AppRoot + `logs`)

	// Initialize all components (config → DB → Redis → Gin)
	initialize.Initialize()

	// Start HTTP server (goroutine)
	go lib_web.WebRun(define.WebService)

	// Listen for system signals and wait for graceful shutdown
	// Buffered so signals are never dropped: signal.Notify does not block when
	// sending, and an unbuffered channel loses signals that arrive before the
	// receive below is reached, breaking graceful shutdown on SIGTERM.
	sc := make(chan os.Signal, 1)
	sl := []os.Signal{
		syscall.SIGHUP,  // hangup
		syscall.SIGINT,  // interrupt (Ctrl+C)
		syscall.SIGTERM, // terminated
	}
	signal.Notify(sc, sl...)
	sig := <-sc
	logs.Info(`received signal: ` + sig.String())

	// Graceful shutdown: stop background tasks before closing the HTTP server.
	initialize.Shutdown()
	lib_web.Shutdown(define.WebService)
	logs.Info(`goteams stopped`)
}
