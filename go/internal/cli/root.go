package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

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
	var host string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		host = args[0]
		args = args[1:]
	}

	fs := flag.NewFlagSet("xp2p ping", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	count := fs.Int("count", 4, "number of echo requests to send")
	timeout := fs.Int("timeout", 0, "per-request timeout in seconds (optional)")
	proto := fs.String("proto", "tcp", "protocol to use (tcp or udp)")
	port := fs.Int("port", 0, "target port (default 62022)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
		return 2
	}

	remaining := fs.Args()
	if host == "" {
		if len(remaining) == 0 {
			fmt.Fprintln(os.Stderr, "xp2p ping: host is required")
			return 2
		}
		host = remaining[0]
		remaining = remaining[1:]
	}
	if len(remaining) > 0 {
		fmt.Fprintf(os.Stderr, "xp2p ping: unexpected arguments: %v\n", remaining)
		return 2
	}

	opts := ping.Options{
		Count:   *count,
		Timeout: time.Duration(*timeout) * time.Second,
		Proto:   *proto,
		Port:    *port,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := ping.Run(ctx, host, opts); err != nil {
		fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
		return 1
	}

	// Align with standard ping exit behaviour where unreachable host returns non-zero.
	return 0
}

func printUsage() {
	fmt.Println(`xp2p - cross-platform helper for XRAY-P2P

Usage:
  xp2p            Start diagnostics service (TCP/UDP port 62022)
  xp2p ping [--proto tcp|udp] [--port PORT] [--count N] [--timeout SECONDS] <host>
`)
}
