package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/NlightN22/xray-p2p/go/internal/cli"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func main() {
	cfg, args, err := parseRootArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printRootUsage()
			return
		}
		fmt.Fprintf(os.Stderr, "xp2p: %v\n", err)
		os.Exit(2)
	}

	logging.Configure(logging.Options{
		Level:  cfg.Logging.Level,
		Format: logFormatFromConfig(cfg.Logging.Format),
	})

	if len(args) == 0 {
		runService(cfg)
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	code := cli.Execute(ctx, cfg, args)
	cancel()
	os.Exit(code)
}

func parseRootArgs(args []string) (config.Config, []string, error) {
	fs := flag.NewFlagSet("xp2p", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	configPath := fs.String("config", "", "path to configuration file")
	logLevel := fs.String("log-level", "", "override logging level")
	serverPort := fs.String("server-port", "", "diagnostics service port")
	serverInstallDir := fs.String("server-install-dir", "", "server installation directory (Windows)")
	serverConfigDir := fs.String("server-config-dir", "", "server configuration directory name")
	serverMode := fs.String("server-mode", "", "server startup mode (auto|manual)")
	serverCert := fs.String("server-cert", "", "path to TLS certificate file (PEM)")
	serverKey := fs.String("server-key", "", "path to TLS private key file (PEM)")
	logJSON := fs.Bool("log-json", false, "emit logs in JSON format")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return config.Config{}, nil, flag.ErrHelp
		}
		return config.Config{}, nil, err
	}

	overrides := make(map[string]any)
	if lvl := strings.TrimSpace(*logLevel); lvl != "" {
		overrides["logging.level"] = lvl
	}
	if port := strings.TrimSpace(*serverPort); port != "" {
		overrides["server.port"] = port
	}
	if dir := strings.TrimSpace(*serverInstallDir); dir != "" {
		overrides["server.install_dir"] = dir
	}
	if cfgDir := strings.TrimSpace(*serverConfigDir); cfgDir != "" {
		overrides["server.config_dir"] = cfgDir
	}
	if mode := strings.TrimSpace(*serverMode); mode != "" {
		overrides["server.mode"] = mode
	}
	if cert := strings.TrimSpace(*serverCert); cert != "" {
		overrides["server.certificate"] = cert
	}
	if key := strings.TrimSpace(*serverKey); key != "" {
		overrides["server.key"] = key
	}
	if *logJSON {
		overrides["logging.format"] = "json"
	}

	cfg, err := config.Load(config.Options{
		Path:      strings.TrimSpace(*configPath),
		Overrides: overrides,
	})
	if err != nil {
		return config.Config{}, nil, err
	}

	return cfg, fs.Args(), nil
}

func runService(cfg config.Config) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := server.StartBackground(ctx, server.Options{Port: cfg.Server.Port}); err != nil {
		logging.Error("failed to start diagnostics service", "err", err)
		os.Exit(1)
	}
	logging.Info("diagnostics service started", "port", cfg.Server.Port)

	<-ctx.Done()
	logging.Info("diagnostics service stopped")
}

func printRootUsage() {
	fmt.Print(`xp2p - cross-platform helper for XRAY-P2P

Usage:
  xp2p [--config FILE] [--log-level LEVEL] [--server-port PORT]
       [--server-install-dir PATH] [--server-config-dir NAME]
       [--server-cert FILE] [--server-key FILE] [--log-json]
  xp2p ping [--proto tcp|udp] [--port PORT] [--count N] [--timeout SECONDS] <host>
  xp2p server install [--path PATH] [--config-dir NAME] [--port PORT]
                      [--cert FILE] [--key FILE]
  xp2p server remove [--path PATH]
  xp2p server run [--path PATH] [--config-dir NAME] [--quiet] [--auto-install]
                  [--xray-log-file FILE]
`)
}

func logFormatFromConfig(value string) logging.Format {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "json":
		return logging.FormatJSON
	default:
		return logging.FormatText
	}
}
