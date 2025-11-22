package servercmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

var (
	serverInstallFunc        = server.Install
	serverRemoveFunc         = server.Remove
	serverRunFunc            = server.Run
	serverUserAddFunc        = server.AddUser
	serverUserRemoveFunc     = server.RemoveUser
	detectPublicHostFunc     = netutil.DetectPublicHost
	serverSetCertFunc        = server.SetCertificate
	serverUserLinkFunc       = server.GetUserLink
	serverUserListFunc       = server.ListUsers
	serverDeployFunc         = runServerDeploy
	serverRedirectAddFunc    = server.AddRedirect
	serverRedirectRemoveFunc = server.RemoveRedirect
	serverRedirectListFunc   = server.ListRedirects
	serverReverseListFunc    = server.ListReverse
)

var promptYesNoFunc = clishared.PromptYesNo

type serverInstallCommandOptions struct {
	Path      string
	ConfigDir string
	Port      string
	Cert      string
	Key       string
	Host      string
	Force     bool
}

type serverRemoveCommandOptions struct {
	Path          string
	ConfigDir     string
	KeepFiles     bool
	IgnoreMissing bool
	Quiet         bool
}

type serverRunCommandOptions struct {
	Path        string
	ConfigDir   string
	AutoInstall bool
	Quiet       bool
	XrayLogFile string
}

func runServerInstall(ctx context.Context, cfg config.Config, opts serverInstallCommandOptions) int {
	installOpts, err := buildInstallOptions(ctx, cfg, opts)
	if err != nil {
		logging.Error("xp2p server install: invalid options", "err", err)
		return 1
	}
	if err := serverInstallFunc(ctx, installOpts); err != nil {
		logging.Error("xp2p server install failed", "err", err)
		return 1
	}

	logging.Info("xp2p server installed", "install_dir", installOpts.InstallDir, "config_dir", installOpts.ConfigDir)

	if strings.TrimSpace(cfg.Client.User) == "" && strings.TrimSpace(cfg.Client.Password) == "" {
		if err := generateDefaultServerCredential(ctx, installOpts, installOpts.Host); err != nil {
			logging.Warn("xp2p server install: failed to generate trojan credential", "err", err)
		}
	}
	return 0
}

func runServerRemove(ctx context.Context, cfg config.Config, opts serverRemoveCommandOptions) int {
	removeOpts := server.RemoveOptions{
		InstallDir:    clishared.FirstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:     clishared.FirstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		KeepFiles:     opts.KeepFiles,
		IgnoreMissing: opts.IgnoreMissing,
	}

	if !opts.Quiet {
		question := fmt.Sprintf("Remove xp2p server installation at %s (%s)?", removeOpts.InstallDir, removeOpts.ConfigDir)
		ok, err := promptYesNoFunc(question)
		if err != nil {
			logging.Error("xp2p server remove: prompt failed", "err", err)
			return 1
		}
		if !ok {
			logging.Info("xp2p server remove aborted by user")
			return 1
		}
	}

	if err := serverRemoveFunc(ctx, removeOpts); err != nil {
		logging.Error("xp2p server remove failed", "err", err)
		return 1
	}
	logging.Info("xp2p server removed", "install_dir", removeOpts.InstallDir, "config_dir", removeOpts.ConfigDir)
	return 0
}

func runServerRun(ctx context.Context, cfg config.Config, opts serverRunCommandOptions) int {
	execOpts, err := prepareRunOptions(ctx, cfg, opts)
	if err != nil {
		logging.Error("xp2p server run: prerequisites failed", "err", err)
		return 1
	}

	cancelDiagnostics := startDiagnostics(ctx, cfg.Server.Port, execOpts.InstallDir)
	if cancelDiagnostics != nil {
		defer cancelDiagnostics()
	}

	if err := serverRunFunc(ctx, execOpts); err != nil {
		logging.Error("xp2p server run failed", "err", err)
		return 1
	}
	return 0
}

func buildInstallOptions(ctx context.Context, cfg config.Config, opts serverInstallCommandOptions) (server.InstallOptions, error) {
	portValue := resolveInstallPort(cfg, opts.Port)
	if err := validatePortValue(portValue); err != nil {
		return server.InstallOptions{}, err
	}

	hostValue, autoDetected, err := determineInstallHost(ctx, opts.Host, cfg.Server.Host)
	if err != nil {
		return server.InstallOptions{}, err
	}
	if autoDetected {
		logging.Info("xp2p server install: detected public host", "host", hostValue)
	}

	return server.InstallOptions{
		InstallDir:      clishared.FirstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:       clishared.FirstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		Port:            portValue,
		CertificateFile: clishared.FirstNonEmpty(opts.Cert, cfg.Server.CertificateFile),
		KeyFile:         clishared.FirstNonEmpty(opts.Key, cfg.Server.KeyFile),
		Host:            hostValue,
		Force:           opts.Force,
	}, nil
}

func prepareRunOptions(ctx context.Context, cfg config.Config, opts serverRunCommandOptions) (server.RunOptions, error) {
	installDir := clishared.FirstNonEmpty(opts.Path, cfg.Server.InstallDir)
	if installDir == "" {
		return server.RunOptions{}, errors.New("installation directory is required")
	}

	configDirName := clishared.FirstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir)
	configDirPath, err := resolveConfigDirPath(installDir, configDirName)
	if err != nil {
		return server.RunOptions{}, err
	}

	if err := ensureServerAssets(ctx, cfg, installDir, configDirName, configDirPath, opts.AutoInstall, opts.Quiet); err != nil {
		return server.RunOptions{}, err
	}

	return server.RunOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		ErrorLogPath: strings.TrimSpace(opts.XrayLogFile),
	}, nil
}

func resolveInstallPort(cfg config.Config, flagPort string) string {
	if value := strings.TrimSpace(flagPort); value != "" {
		return value
	}
	if cfgPort := strings.TrimSpace(cfg.Server.Port); cfgPort != "" && cfgPort != server.DefaultPort {
		return cfgPort
	}
	return strconv.Itoa(server.DefaultTrojanPort)
}
