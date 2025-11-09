package root

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
)

const socksConfigSentinel = "__xp2p_socks_config__"

type pingCommandOptions struct {
	Host          string
	Count         int
	TimeoutSec    int
	Proto         string
	Port          int
	SocksEndpoint string
}

func newPingCommand(cfg func() config.Config) *cobra.Command {
	opts := pingCommandOptions{
		Count:      4,
		Proto:      "tcp",
		TimeoutSec: 0,
	}

	cmd := &cobra.Command{
		Use:   "ping <host>",
		Short: "Send diagnostic ping requests to xp2p agents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Host = args[0]
			code := runPingCommand(commandContext(cmd), cfg(), opts)
			if code != 0 {
				return exitError{code: code}
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.Count, "count", opts.Count, "number of echo requests to send")
	flags.IntVar(&opts.TimeoutSec, "timeout", opts.TimeoutSec, "per-request timeout in seconds (optional)")
	flags.StringVar(&opts.Proto, "proto", opts.Proto, "protocol to use (tcp or udp)")
	flags.IntVar(&opts.Port, "port", opts.Port, "target port (default 62022)")
	flags.StringVar(&opts.SocksEndpoint, "socks", "", "route ping through SOCKS5 proxy (host:port); omit value to use config default")
	flags.Lookup("socks").NoOptDefVal = socksConfigSentinel
	return cmd
}

func runPingCommand(ctx context.Context, cfg config.Config, opts pingCommandOptions) int {
	host := strings.TrimSpace(opts.Host)
	if host == "" {
		fmt.Fprintln(os.Stderr, "xp2p ping: host is required")
		return 2
	}

	pingOpts := ping.Options{
		Count:   opts.Count,
		Timeout: time.Duration(opts.TimeoutSec) * time.Second,
		Proto:   strings.TrimSpace(opts.Proto),
		Port:    opts.Port,
	}

	socksAddr, err := resolveSocksAddress(cfg.Client.SocksAddress, opts.SocksEndpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
		return 2
	}
	pingOpts.SocksProxy = socksAddr

	if err := ping.Run(ctx, host, pingOpts); err != nil {
		fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
		return 1
	}
	return 0
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
