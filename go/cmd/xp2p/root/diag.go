package root

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

type diagCommandOptions struct {
	Listen string
	Proto  string
	Quiet  bool
}

func newDiagCommand(cfg func() config.Config) *cobra.Command {
	opts := diagCommandOptions{
		Listen: net.JoinHostPort("0.0.0.0", server.DefaultPort),
		Proto:  "tcp",
	}

	cmd := &cobra.Command{
		Use:   "diag",
		Short: "Run diagnostics responder in the foreground",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("listen") {
				port := strings.TrimSpace(cfg().Server.Port)
				if port == "" {
					port = server.DefaultPort
				}
				opts.Listen = net.JoinHostPort("0.0.0.0", port)
			}
			code := runDiagCommand(commandContext(cmd), cfg(), opts)
			if code != 0 {
				return exitError{code: code}
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Listen, "listen", "n", opts.Listen, "listen address (host:port)")
	flags.StringVarP(&opts.Proto, "proto", "o", opts.Proto, "protocol to listen on (tcp or udp)")
	flags.BoolVarP(&opts.Quiet, "quiet", "q", false, "reduce log output")
	return cmd
}

func runDiagCommand(ctx context.Context, cfg config.Config, opts diagCommandOptions) int {
	listen := strings.TrimSpace(opts.Listen)
	if listen == "" {
		port := strings.TrimSpace(cfg.Server.Port)
		if port == "" {
			port = server.DefaultPort
		}
		listen = net.JoinHostPort("0.0.0.0", port)
	}

	host, port, err := splitListenAddress(listen)
	if err != nil {
		logging.Error("xp2p diag: invalid listen address", "err", err)
		return 2
	}

	proto := strings.ToLower(strings.TrimSpace(opts.Proto))
	if proto == "" {
		proto = "tcp"
	}
	if proto != "tcp" && proto != "udp" {
		logging.Error("xp2p diag: invalid protocol", "proto", opts.Proto)
		return 2
	}

	listenAddr := net.JoinHostPort(host, port)
	if err := server.StartBackground(ctx, server.Options{
		ListenAddr: listenAddr,
		Proto:      proto,
		InstallDir: cfg.Server.InstallDir,
		Quiet:      opts.Quiet,
	}); err != nil {
		logging.Error("xp2p diag: failed to start diagnostics listener", "err", err)
		return 1
	}
	logging.Info("xp2p diagnostics server started", "proto", proto, "listen", listenAddr)
	<-ctx.Done()
	logging.Info("xp2p diagnostics server stopped")
	return 0
}

func splitListenAddress(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", errors.New("listen address is required")
	}
	if strings.HasPrefix(value, "[") {
		host, port, err := net.SplitHostPort(value)
		if err != nil {
			return "", "", fmt.Errorf("invalid listen address %q: %w", value, err)
		}
		if err := validateListenPort(port); err != nil {
			return "", "", err
		}
		return host, port, nil
	}

	if !strings.Contains(value, ":") {
		if err := validateListenPort(value); err != nil {
			return "", "", err
		}
		return "0.0.0.0", value, nil
	}

	idx := strings.LastIndex(value, ":")
	host := strings.TrimSpace(value[:idx])
	port := strings.TrimSpace(value[idx+1:])
	if host == "" {
		host = "0.0.0.0"
	}
	if err := validateListenPort(port); err != nil {
		return "", "", err
	}
	return host, port, nil
}

func validateListenPort(value string) error {
	port := strings.TrimSpace(value)
	if port == "" {
		return errors.New("listen port is empty")
	}
	parsed, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid listen port %q: %w", value, err)
	}
	if parsed <= 0 || parsed > 65535 {
		return fmt.Errorf("invalid listen port %q: must be within 1-65535", value)
	}
	return nil
}
