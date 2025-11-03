package cli

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
)

// Execute parses CLI arguments and runs the corresponding subcommand.
func Execute(ctx context.Context, cfg config.Config, args []string) int {
	if len(args) == 0 {
		printUsage()
		return 0
	}

	switch args[0] {
	case "ping":
		return runPing(ctx, cfg, args[1:])
	case "server":
		return runServer(ctx, cfg, args[1:])
	case "client":
		return runClient(ctx, cfg, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "xp2p: unknown command %q\n\n", args[0])
		printUsage()
		return 1
	}
}

const socksConfigSentinel = "__xp2p_socks_config__"

func runPing(ctx context.Context, cfg config.Config, args []string) int {
	host, opts, err := parsePingArgs(cfg, args)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
		return 2
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

func parsePingArgs(cfg config.Config, args []string) (string, ping.Options, error) {
	var host string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		host = args[0]
		args = args[1:]
	}

	args = normalizeSocksArgs(args)

	fs := flag.NewFlagSet("xp2p ping", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	count := fs.Int("count", 4, "number of echo requests to send")
	timeout := fs.Int("timeout", 0, "per-request timeout in seconds (optional)")
	proto := fs.String("proto", "tcp", "protocol to use (tcp or udp)")
	port := fs.Int("port", 0, "target port (default 62022)")
	socks := fs.String("socks", "", "route ping through SOCKS5 proxy (host:port); omit value to use config default")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return "", ping.Options{}, flag.ErrHelp
		}
		return "", ping.Options{}, err
	}

	remaining := fs.Args()
	if host == "" {
		if len(remaining) == 0 {
			return "", ping.Options{}, fmt.Errorf("host is required")
		}
		host = remaining[0]
		remaining = remaining[1:]
	}
	if len(remaining) > 0 {
		return "", ping.Options{}, fmt.Errorf("unexpected arguments: %v", remaining)
	}

	opts := ping.Options{
		Count:   *count,
		Timeout: time.Duration(*timeout) * time.Second,
		Proto:   *proto,
		Port:    *port,
	}

	socksAddr, err := resolveSocksAddress(cfg.Client.SocksAddress, *socks)
	if err != nil {
		return "", ping.Options{}, err
	}
	opts.SocksProxy = socksAddr

	return host, opts, nil
}

func normalizeSocksArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--socks" {
			value := socksConfigSentinel
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				value = args[i+1]
				i++
			}
			normalized = append(normalized, "--socks="+value)
			continue
		}
		if strings.HasPrefix(arg, "--socks=") {
			if strings.TrimPrefix(arg, "--socks=") == "" {
				arg = "--socks=" + socksConfigSentinel
			}
		}
		normalized = append(normalized, arg)
	}
	return normalized
}

func resolveSocksAddress(configValue, flagValue string) (string, error) {
	value := strings.TrimSpace(flagValue)
	switch value {
	case "":
		return "", nil
	case socksConfigSentinel:
		value = strings.TrimSpace(configValue)
		if value == "" {
			return "", fmt.Errorf("SOCKS proxy address is not configured; provide host:port to --socks")
		}
	}

	host, port, err := splitHostPort(value)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(host, port), nil
}

func splitHostPort(value string) (string, string, error) {
	if strings.HasPrefix(value, "[") {
		host, port, err := net.SplitHostPort(value)
		if err != nil {
			return "", "", fmt.Errorf("invalid SOCKS proxy address %q: %w", value, err)
		}
		if err := validatePort(port); err != nil {
			return "", "", err
		}
		return host, port, nil
	}

	idx := strings.LastIndex(value, ":")
	if idx == -1 {
		return "", "", fmt.Errorf("invalid SOCKS proxy address %q: expected host:port", value)
	}
	host := strings.TrimSpace(value[:idx])
	port := strings.TrimSpace(value[idx+1:])
	if host == "" {
		return "", "", fmt.Errorf("invalid SOCKS proxy address %q: host is empty", value)
	}
	if err := validatePort(port); err != nil {
		return "", "", err
	}
	return host, port, nil
}

func validatePort(port string) error {
	p, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid SOCKS proxy port %q: %w", port, err)
	}
	if p <= 0 || p > 65535 {
		return fmt.Errorf("invalid SOCKS proxy port %q: must be within 1-65535", port)
	}
	return nil
}

func printUsage() {
	fmt.Print(`xp2p - cross-platform helper for XRAY-P2P

Usage:
  xp2p            Start diagnostics service (TCP/UDP port 62022)
  xp2p ping [--proto tcp|udp] [--port PORT] [--count N] [--timeout SECONDS]
            [--socks [HOST:PORT]] <host>
  xp2p server install [--path PATH] [--config-dir NAME] [--port PORT] [--cert FILE] [--key FILE]
  xp2p server remove  [--path PATH]
  xp2p server run     [--path PATH] [--config-dir NAME] [--quiet] [--auto-install]
  xp2p client install [--path PATH] [--config-dir NAME] --server-address HOST --password SECRET
                      [--server-port PORT] [--server-name NAME]
                      [--allow-insecure|--strict-tls] [--force]
  xp2p client deploy  --remote-host HOST [--ssh-user NAME] [--ssh-port PORT]
                      [--server-host HOST] [--server-port PORT]
                      [--user EMAIL] [--password SECRET]
                      [--install-dir PATH] [--config-dir NAME]
                      [--local-install PATH] [--local-config NAME]
                      [--save-link FILE]
  xp2p client remove  [--path PATH]
  xp2p client run     [--path PATH] [--config-dir NAME] [--quiet] [--auto-install]
                      (requires client server address and password configured)
`)
}
