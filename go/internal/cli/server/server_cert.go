package servercmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func runServerCert(ctx context.Context, cfg config.Config, args []string) int {
	if len(args) == 0 {
		printServerCertUsage()
		return 1
	}

	switch strings.ToLower(args[0]) {
	case "set":
		return runServerCertSet(ctx, cfg, args[1:])
	case "-h", "--help", "help":
		printServerCertUsage()
		return 0
	default:
		logging.Error("xp2p server cert: unknown subcommand", "subcommand", args[0])
		printServerCertUsage()
		return 1
	}
}

func runServerCertSet(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server cert set", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name or absolute path")
	cert := fs.String("cert", "", "TLS certificate file to deploy")
	key := fs.String("key", "", "TLS private key file to deploy")
	host := fs.String("host", "", "public host name or IP for certificate generation")
	force := fs.Bool("force", false, "overwrite existing TLS configuration without prompting")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server cert set: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server cert set: unexpected arguments", "args", fs.Args())
		return 2
	}

	explicitHost := strings.TrimSpace(*host)
	hostValue := explicitHost
	autoDetected := false
	if hostValue == "" && strings.TrimSpace(*cert) == "" {
		value, detected, err := determineInstallHost(ctx, "", cfg.Server.Host)
		if err != nil {
			logging.Error("xp2p server cert set: failed to resolve public host", "err", err)
			return 1
		}
		hostValue = value
		autoDetected = detected
	}
	if autoDetected {
		logging.Info("xp2p server cert set: detected public host", "host", hostValue)
	}

	opts := server.CertificateOptions{
		InstallDir:      firstNonEmpty(*path, cfg.Server.InstallDir),
		ConfigDir:       firstNonEmpty(*configDir, cfg.Server.ConfigDir),
		CertificateFile: strings.TrimSpace(*cert),
		KeyFile:         strings.TrimSpace(*key),
		Host:            hostValue,
		Force:           *force,
	}

	return applyServerCertificate(ctx, opts)
}

func applyServerCertificate(ctx context.Context, opts server.CertificateOptions) int {
	err := serverSetCertFunc(ctx, opts)
	if err == nil {
		logging.Info("xp2p server cert set completed",
			"install_dir", opts.InstallDir,
			"config_dir", opts.ConfigDir,
		)
		return 0
	}

	if errors.Is(err, server.ErrCertificateConfigured) && !opts.Force {
		ok, promptErr := promptYesNoFunc("TLS already configured. Replace existing certificate?")
		if promptErr != nil {
			logging.Error("xp2p server cert set: prompt failed", "err", promptErr)
			return 1
		}
		if !ok {
			logging.Info("xp2p server cert set cancelled by user")
			return 0
		}

		opts.Force = true
		if err = serverSetCertFunc(ctx, opts); err == nil {
			logging.Info("xp2p server cert set completed",
				"install_dir", opts.InstallDir,
				"config_dir", opts.ConfigDir,
			)
			return 0
		}
	}

	logging.Error("xp2p server cert set failed", "err", err)
	return 1
}

func printServerCertUsage() {
	fmt.Print(`xp2p server cert commands:
  set [--path PATH] [--config-dir NAME|PATH] [--cert FILE] [--key FILE]
      [--host HOST] [--force]
`)
}
