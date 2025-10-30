package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/NlightN22/xray-p2p/go/internal/cli"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func main() {
	if len(os.Args) <= 1 {
		runService()
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	code := cli.Execute(ctx, os.Args[1:])
	cancel()
	os.Exit(code)
}

func runService() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := server.StartBackground(ctx); err != nil {
		logging.Error("failed to start diagnostics service", "err", err)
		os.Exit(1)
	}
	logging.Info("diagnostics service started", "port", server.DefaultPort)

	<-ctx.Done()
	logging.Info("diagnostics service stopped")
}
