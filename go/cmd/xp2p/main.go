package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/NlightN22/xray-p2p/go/internal/cli"
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
		log.Fatalf("xp2p: failed to start diagnostics service: %v", err)
	}
	log.Printf("xp2p service started (TCP/UDP port %s). Press Ctrl+C to stop.", server.DefaultPort)

	<-ctx.Done()
	log.Println("xp2p service stopped.")
}
