package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

const tunnelConfigSentinel = "__xp2p_tunnel_config__"

type pingCommandOptions struct {
	Host           string
	Count          int
	TimeoutSec     int
	Proto          string
	Port           int
	TunnelEndpoint string
	EndpointTag    string
	EndpointIndex  int
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
	flags.IntVarP(&opts.Count, "count", "N", opts.Count, "number of echo requests to send")
	flags.IntVarP(&opts.TimeoutSec, "timeout", "t", opts.TimeoutSec, "per-request timeout in seconds (optional)")
	flags.StringVarP(&opts.Proto, "proto", "o", opts.Proto, "protocol to use (tcp or udp)")
	flags.IntVarP(&opts.Port, "port", "P", opts.Port, "target port (default 62022)")
	flags.StringVarP(&opts.TunnelEndpoint, "tunnel", "T", "", "route ping through xp2p tunnel (SOCKS5 host:port); omit value to auto-detect from xp2p config")
	flags.StringVarP(&opts.EndpointTag, "endpoint", "e", "", "endpoint tag to use when multiple endpoints share the same host")
	flags.IntVarP(&opts.EndpointIndex, "index", "i", 0, "endpoint index (1-based) to use when multiple endpoints share the same host")
	flags.Lookup("tunnel").NoOptDefVal = tunnelConfigSentinel
	return cmd
}

func runPingCommand(ctx context.Context, cfg config.Config, opts pingCommandOptions) int {
	host := strings.TrimSpace(opts.Host)
	if host == "" {
		fmt.Fprintln(os.Stderr, "xp2p ping: host is required")
		return 2
	}

	autoTunnel := strings.TrimSpace(opts.TunnelEndpoint) == tunnelConfigSentinel
	var clientSocksAddr string
	var serverSocksAddr string

	pingOpts := ping.Options{
		Count:   opts.Count,
		Timeout: time.Duration(opts.TimeoutSec) * time.Second,
		Proto:   strings.TrimSpace(opts.Proto),
		Port:    opts.Port,
	}

	var socksAddr string
	var err error
	if autoTunnel {
		clientSocksAddr, serverSocksAddr, err = detectSocksProxies(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
			return 2
		}
		socksAddr, err = resolveAutoSocks(false, clientSocksAddr, serverSocksAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
			return 2
		}
	} else {
		socksAddr, err = resolveSocksAddress(cfg, opts.TunnelEndpoint)
		if err != nil {
			fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
			return 2
		}
	}

	hasEndpointSelector := strings.TrimSpace(opts.EndpointTag) != "" || opts.EndpointIndex > 0
	if socksAddr == "" && hasEndpointSelector {
		fmt.Fprintln(os.Stderr, "xp2p ping: --endpoint/--index requires --tunnel")
		return 2
	}
	if socksAddr != "" {
		if !strings.EqualFold(strings.TrimSpace(pingOpts.Proto), "tcp") {
			fmt.Fprintln(os.Stderr, "xp2p ping: --tunnel supports only tcp protocol")
			return 2
		}
		markerTarget, markerPort, markerErr := client.ResolveMarkerTarget(cfg.Client.InstallDir, host, opts.EndpointTag, opts.EndpointIndex)
		useMarker := markerErr == nil
		usedServerMarker := false
		if markerErr != nil && hasEndpointSelector {
			if errors.Is(markerErr, client.ErrClientEndpointsMissing) || errors.Is(markerErr, client.ErrClientEndpointNotFound) {
				markerTarget, markerPort, markerErr = server.ResolveServerMarkerTarget(cfg.Server.InstallDir, opts.EndpointTag, opts.EndpointIndex)
				useMarker = markerErr == nil
				usedServerMarker = markerErr == nil
			}
		} else if markerErr != nil && !hasEndpointSelector {
			if errors.Is(markerErr, client.ErrClientEndpointsMissing) || errors.Is(markerErr, client.ErrClientEndpointNotFound) {
				markerTarget, markerPort, markerErr = server.ResolveServerMarkerTarget(cfg.Server.InstallDir, host, 0)
				useMarker = markerErr == nil
				usedServerMarker = markerErr == nil
				if markerErr != nil && isIgnorableServerResolveError(markerErr) {
					markerErr = nil
				}
			}
		}
		if markerErr != nil {
			fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", markerErr)
			return 2
		}
		if useMarker {
			host = markerTarget
			pingOpts.Proto = "tcp"
			pingOpts.Port = markerPort
		}
		if autoTunnel && usedServerMarker {
			socksAddr, err = resolveAutoSocks(true, clientSocksAddr, serverSocksAddr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
				return 2
			}
		}
		pingOpts.SocksProxy = socksAddr
	} else {
		pingOpts.SocksProxy = ""
	}

	if err := ping.Run(ctx, host, pingOpts); err != nil {
		fmt.Fprintf(os.Stderr, "xp2p ping: %v\n", err)
		return 1
	}
	return 0
}
