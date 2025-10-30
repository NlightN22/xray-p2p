package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
)

// Execute parses CLI arguments and runs the corresponding subcommand.
func Execute(ctx context.Context, args []string) int {
	if len(args) == 0 {
		printUsage()
		return 0
	}

	switch args[0] {
	case "ping":
		return runPing(ctx, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "xp2p: unknown command %q\n\n", args[0])
		printUsage()
		return 1
	}
}

func runPing(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("xp2p ping", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	count := fs.Int("count", 4, "number of echo requests to send")
	timeout := fs.Int("timeout", 0, "per-request timeout in seconds (optional)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(os.Stderr, "xp2p ping: host is required")
		return 2
	}

	target := remaining[0]
	opts := ping.Options{
		Count:   *count,
		Timeout: *timeout,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := ping.Run(ctx, target, opts); err != nil {
		fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
		return 1
	}

	// Align with standard ping exit behaviour where unreachable host returns non-zero.
	return 0
}

func printUsage() {
	fmt.Println(`xp2p - cross-platform helper for XRAY-P2P

Usage:
  xp2p ping [--count N] [--timeout SECONDS] <host>
`)
}
