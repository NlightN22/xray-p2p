package root

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
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
	flags.StringVar(&opts.SocksEndpoint, "socks", "", "route ping through SOCKS5 proxy (host:port); omit value to auto-detect from xp2p config (client first, then server)")
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

	socksAddr, err := resolveSocksAddress(cfg, opts.SocksEndpoint)
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

func resolveSocksAddress(cfg config.Config, flagValue string) (string, error) {
	value := strings.TrimSpace(flagValue)
	switch value {
	case "":
		return "", nil
	case socksConfigSentinel:
		return detectSocksProxy(cfg)
	}

	return normalizeSocksAddress(value)
}

func normalizeSocksAddress(value string) (string, error) {
	host, port, err := splitHostPort(value)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(host, port), nil
}

var errSocksInboundNotFound = errors.New("socks inbound not found")

func detectSocksProxy(cfg config.Config) (string, error) {
	if addr, err := loadSocksAddress(cfg.Client.InstallDir, cfg.Client.ConfigDir); err == nil {
		return addr, nil
	} else if !errors.Is(err, errSocksInboundNotFound) {
		return "", err
	}

	if addr, err := loadSocksAddress(cfg.Server.InstallDir, cfg.Server.ConfigDir); err == nil {
		return addr, nil
	} else if !errors.Is(err, errSocksInboundNotFound) {
		return "", err
	}

	return "", fmt.Errorf("SOCKS proxy not configured; specify --socks host:port or install xp2p client/server")
}

func loadSocksAddress(installDir, configDir string) (string, error) {
	dir := strings.TrimSpace(configDir)
	if dir == "" {
		return "", errSocksInboundNotFound
	}
	if !filepath.IsAbs(dir) {
		base := strings.TrimSpace(installDir)
		if base == "" {
			return "", errSocksInboundNotFound
		}
		dir = filepath.Join(base, dir)
	}

	path := filepath.Join(dir, "inbounds.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errSocksInboundNotFound
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}

	raw, ok := root["inbounds"]
	if !ok {
		return "", fmt.Errorf("%s missing \"inbounds\" array", path)
	}
	entries, ok := raw.([]any)
	if !ok {
		return "", fmt.Errorf("%s has invalid \"inbounds\" array", path)
	}

	for _, entryRaw := range entries {
		entry, ok := entryRaw.(map[string]any)
		if !ok {
			continue
		}
		proto, _ := entry["protocol"].(string)
		if !strings.EqualFold(strings.TrimSpace(proto), "socks") {
			continue
		}

		host := ""
		if listenRaw, ok := entry["listen"]; ok {
			value, err := stringifyListen(listenRaw)
			if err != nil {
				return "", fmt.Errorf("%s: %w", path, err)
			}
			host = value
		}
		if host == "" {
			host = "127.0.0.1"
		}

		port, err := parseInboundPort(entry["port"])
		if err != nil {
			return "", fmt.Errorf("%s: %w", path, err)
		}
		return net.JoinHostPort(host, port), nil
	}

	return "", errSocksInboundNotFound
}

func stringifyListen(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("invalid SOCKS listen value of type %T", value)
	}
	return strings.TrimSpace(str), nil
}

func parseInboundPort(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", errors.New("SOCKS inbound is missing \"port\"")
	case json.Number:
		return normalizePortString(v.String())
	case float64:
		return normalizePortInt(int(v), v == float64(int(v)))
	case string:
		return normalizePortString(v)
	default:
		return "", fmt.Errorf("invalid SOCKS inbound port value of type %T", value)
	}
}

func normalizePortString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("SOCKS inbound port is empty")
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return "", fmt.Errorf("invalid SOCKS inbound port %q: %w", raw, err)
	}
	if val <= 0 || val > 65535 {
		return "", fmt.Errorf("invalid SOCKS inbound port %q: must be within 1-65535", raw)
	}
	return strconv.Itoa(val), nil
}

func normalizePortInt(value int, exact bool) (string, error) {
	if !exact {
		return "", fmt.Errorf("invalid SOCKS inbound port %d: not an integer", value)
	}
	if value <= 0 || value > 65535 {
		return "", fmt.Errorf("invalid SOCKS inbound port %d: must be within 1-65535", value)
	}
	return strconv.Itoa(value), nil
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
