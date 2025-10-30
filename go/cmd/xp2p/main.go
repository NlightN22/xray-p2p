package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/NlightN22/xray-p2p/go/internal/cli"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server.StartBackground(ctx)

	code := cli.Execute(ctx, os.Args[1:])
	cancel()
	os.Exit(code)
}
