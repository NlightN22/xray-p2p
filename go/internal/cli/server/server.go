package servercmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

var (
	serverInstallFunc        = server.Install
	serverRemoveFunc         = server.Remove
	serverRunFunc            = server.Run
	serverServiceRunFunc     = server.RunService
	serverUserAddFunc        = server.AddUser
	serverUserStageFunc      = server.StageUser
	serverUserRemoveFunc     = server.RemoveUser
	serverUserUpdateFunc     = server.UpdateUser
	serverUserRotateFunc     = server.RotateUser
	serverSetProfileFunc     = server.SetProfile
	detectPublicHostFunc     = netutil.DetectPublicHost
	serverSetCertFunc        = server.SetCertificate
	serverCertStateFunc      = server.CertificateStateFromConfig
	serverUserLinkFunc       = server.GetUserLink
	serverUserListFunc       = server.ListUsers
	serverDeployFunc         = runServerDeploy
	serverRedirectAddFunc    = server.AddRedirect
	serverRedirectRemoveFunc = server.RemoveRedirect
	serverRedirectListFunc   = server.ListRedirects
	serverRedirectToggleFunc = server.SetRedirectEnabled
	serverReverseListFunc    = server.ListReverse
	serverReverseToggleFunc  = server.SetReverseEnabled
)

var promptYesNoFunc = clishared.PromptYesNo
var promptChoiceFunc = clishared.PromptChoice

// SetInstallForTesting replaces the server install boundary until the returned
// restore function is called.
func SetInstallForTesting(fn func(context.Context, server.InstallOptions) error) func() {
	previous := serverInstallFunc
	serverInstallFunc = fn
	return func() { serverInstallFunc = previous }
}

// SetRemoveForTesting replaces the removal boundary until the returned restore
// function is called.
func SetRemoveForTesting(fn func(context.Context, server.RemoveOptions) error) func() {
	previous := serverRemoveFunc
	serverRemoveFunc = fn
	return func() { serverRemoveFunc = previous }
}

// SetCertificateForTesting replaces the certificate boundary until the
// returned restore function is called.
func SetCertificateForTesting(fn func(context.Context, server.CertificateOptions) error) func() {
	previous := serverSetCertFunc
	serverSetCertFunc = fn
	return func() { serverSetCertFunc = previous }
}

// SetUserAddForTesting replaces the user creation boundary until the returned
// restore function is called.
func SetUserAddForTesting(fn func(context.Context, server.AddUserOptions) error) func() {
	previous := serverUserAddFunc
	serverUserAddFunc = fn
	return func() { serverUserAddFunc = previous }
}

// SetUserLinkForTesting replaces the credential-link boundary until the returned
// restore function is called.
func SetUserLinkForTesting(
	fn func(context.Context, server.UserLinkOptions) (server.UserLink, error),
) func() {
	previous := serverUserLinkFunc
	serverUserLinkFunc = fn
	return func() { serverUserLinkFunc = previous }
}

type serverInstallCommandOptions struct {
	Path      string
	ConfigDir string
	Port      string
	CertStore string
	Cert      string
	Key       string
	Host      string
	Profile   string
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
	DiagPort    string
	DiagMode    string
	AutoInstall bool
	Quiet       bool
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

	if port := strings.TrimSpace(installOpts.Port); port != "" {
		if _, err := config.UpdateServerTrojanPort("", port); err != nil {
			logging.Warn("xp2p server install: failed to update server trojan port", "err", err)
		} else {
			if req, reqErr := apply.NewRequest(apply.RoleServer); reqErr == nil {
				_ = apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
			}
		}
	}

	var generated *credentialResult
	warnings := make([]string, 0)
	if strings.TrimSpace(cfg.Client.User) == "" && strings.TrimSpace(cfg.Client.Password) == "" {
		result, err := generateDefaultServerCredential(ctx, installOpts, installOpts.Host)
		if err != nil {
			logging.Warn("xp2p server install: failed to generate server credential", "err", err)
			warnings = append(warnings, "failed to generate server credential")
		} else {
			generated = &result
			if !clioutput.EnabledContext(ctx) {
				announceCredential("Generated server credential", result)
			}
		}
	}
	if clioutput.EnabledContext(ctx) {
		type credentialOutput struct {
			User     string  `json:"user"`
			Password string  `json:"password"`
			Link     *string `json:"link"`
		}
		var credential *credentialOutput
		if generated != nil {
			var link *string
			if generated.linkErr == nil && strings.TrimSpace(generated.details.Link) != "" {
				value := generated.details.Link
				link = &value
			}
			credential = &credentialOutput{
				User: generated.details.UserID, Password: generated.details.Password, Link: link,
			}
		}
		result := struct {
			Status     string            `json:"status"`
			InstallDir string            `json:"install_dir"`
			ConfigDir  string            `json:"config_dir"`
			Credential *credentialOutput `json:"credential"`
			Warnings   []string          `json:"warnings"`
		}{
			Status: "completed", InstallDir: installOpts.InstallDir, ConfigDir: installOpts.ConfigDir,
			Credential: credential, Warnings: warnings,
		}
		if err := clioutput.SetResultContext(ctx, result); err != nil {
			logging.Error("xp2p server install: publish JSON result failed", "err", err)
			return 1
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
		TunName:       cfg.Server.TunName,
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
	logging.Warn("xp2p server remove: stop the server service if it is running (xp2p server service stop)")
	return 0
}

func runServerRun(ctx context.Context, cfg config.Config, opts serverRunCommandOptions) int {
	if port := strings.TrimSpace(opts.DiagPort); port != "" {
		cfg.Server.Port = port
	}
	if mode := strings.TrimSpace(opts.DiagMode); mode != "" {
		cfg.Server.Mode = mode
	}

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
	profileValue, err := normalizeServerInstallProfile(clishared.FirstNonEmpty(opts.Profile, cfg.Server.Profile))
	if err != nil {
		return server.InstallOptions{}, err
	}

	return server.InstallOptions{
		InstallDir:       clishared.FirstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:        clishared.FirstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		Port:             portValue,
		CertificateStore: clishared.FirstNonEmpty(opts.CertStore, cfg.Server.CertificateStore),
		CertificateFile:  clishared.FirstNonEmpty(opts.Cert, cfg.Server.CertificateFile),
		KeyFile:          clishared.FirstNonEmpty(opts.Key, cfg.Server.KeyFile),
		Host:             hostValue,
		Profile:          profileValue,
		Force:            opts.Force,
		TunEnabled:       cfg.Server.TunEnabled,
		TunEnabledSet:    true,
		TunName:          cfg.Server.TunName,
		TunMTU:           cfg.Server.TunMTU,
		TunAddr:          cfg.Server.TunAddr,
	}, nil
}

func normalizeServerInstallProfile(value string) (string, error) {
	endpoint, err := tunnel.DefaultProfile(tunnel.Profile(strings.TrimSpace(value)))
	if err != nil {
		return "", fmt.Errorf("invalid server profile: %w", err)
	}
	return string(endpoint.Profile), nil
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
	if err := ensureServerApplyRequestIfDesiredOnly(); err != nil {
		return server.RunOptions{}, err
	}

	return server.RunOptions{
		InstallDir: installDir,
		ConfigDir:  configDirName,
		TunEnabled: cfg.Server.TunEnabled,
		TunName:    cfg.Server.TunName,
		TunMTU:     cfg.Server.TunMTU,
		TunAddr:    cfg.Server.TunAddr,
	}, nil
}

func resolveInstallPort(cfg config.Config, flagPort string) string {
	if value := strings.TrimSpace(flagPort); value != "" {
		return value
	}
	if cfgPort := strings.TrimSpace(cfg.Server.TrojanPort); cfgPort != "" {
		return cfgPort
	}
	return strconv.Itoa(server.DefaultTrojanPort)
}
