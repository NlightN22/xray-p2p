package servercmd

import (
	"context"
	"errors"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

type serverCertSetOptions struct {
	Path      string
	ConfigDir string
	Cert      string
	Key       string
	Host      string
	Force     bool
}

func runServerCertSet(ctx context.Context, cfg config.Config, opts serverCertSetOptions) int {
	explicitHost := strings.TrimSpace(opts.Host)
	hostValue := explicitHost
	autoDetected := false
	if hostValue == "" && strings.TrimSpace(opts.Cert) == "" {
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

	certOpts := server.CertificateOptions{
		InstallDir:      firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:       firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		CertificateFile: strings.TrimSpace(opts.Cert),
		KeyFile:         strings.TrimSpace(opts.Key),
		Host:            hostValue,
		Force:           opts.Force,
	}

	return applyServerCertificate(ctx, certOpts)
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
