package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	rootcmd "github.com/NlightN22/xray-p2p/go/cmd/xp2p/root"
	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeprobe"
)

type renderedError interface {
	Rendered() bool
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	runtimeprobe.Start(ctx)

	args := os.Args[1:]
	cmd := rootcmd.NewCommandForArgs(args)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		var exitCode interface{ ExitCode() int }
		if errors.As(err, &exitCode) {
			os.Exit(rootcmd.ProcessExitCode(err))
		}
		var rendered renderedError
		if errors.As(err, &rendered) && rendered.Rendered() {
			os.Exit(1)
		}
		if jsonOutputRequested(os.Args[1:]) {
			_ = clioutput.WriteError(os.Stderr, cmd.CommandPath(), "command_failed", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func jsonOutputRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" || arg == "-J" || strings.EqualFold(arg, "--json=true") {
			return true
		}
	}
	return false
}
