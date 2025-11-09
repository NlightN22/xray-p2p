package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	rootcmd "github.com/NlightN22/xray-p2p/go/cmd/xp2p/root"
)

type exitCoder interface {
	ExitCode() int
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := rootcmd.NewCommand()
	cmd.SetArgs(os.Args[1:])
	if err := cmd.ExecuteContext(ctx); err != nil {
		var ec exitCoder
		if errors.As(err, &ec) {
			os.Exit(ec.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "xp2p: %v\n", err)
		os.Exit(1)
	}
}
