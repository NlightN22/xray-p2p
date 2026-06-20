//go:build linux

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/extensions"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/openwrt"
	"github.com/NlightN22/xray-p2p/go/internal/service/control"
)

type installState struct {
	InstallOptions
	installDir string
	configDir  string
	logsDir    string
	serverPort int
	serverName string
	serverHost string
	configFile string
	stateFile  string
}

// Install deploys client configuration files on Linux/OpenWrt hosts.
func Install(ctx context.Context, opts InstallOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	state, err := normalizeInstallOptions(opts)
	if err != nil {
		return err
	}

	logging.Info("xp2p client install starting",
		"install_dir", state.installDir,
		"config_dir", state.configDir,
		"server_address", state.ServerAddress,
		"server_port", state.serverPort,
		"allow_insecure", state.AllowInsecure,
	)

	if err := os.MkdirAll(state.configDir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	logRoot := config.LogRoot()
	if err := os.MkdirAll(logRoot, 0o777); err != nil {
		return fmt.Errorf("create log root: %w", err)
	}
	if err := os.Chmod(logRoot, 0o777); err != nil {
		logging.Warn("chmod log root failed", "path", logRoot, "err", err)
	}
	if err := os.MkdirAll(state.logsDir, 0o777); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	if err := os.Chmod(state.logsDir, 0o777); err != nil {
		logging.Warn("chmod log directory failed", "path", state.logsDir, "err", err)
	}
	if !state.Force {
		exists, err := clientEndpointConfigured(state.configFile, state.serverHost, state.serverPort)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("endpoint %s:%d already exists (use --force to update)", state.serverHost, state.serverPort)
		}
	}
	if err := ensureClientTunConfig(state.Force, state.TunEnabled, state.TunName, state.TunMTU, state.TunAddr, state.TunMode, state.TunModeSet); err != nil {
		return err
	}
	if state.TunEnabled {
		if err := openwrt.EnsureTunInterface(state.TunName, state.TunAddr); err != nil {
			return err
		}
	}

	if err := deployDesiredConfiguration(ctx, state); err != nil {
		return err
	}
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		return err
	}
	logging.Info("xp2p client install completed", "install_dir", state.installDir)
	return nil
}

// Remove deletes client configuration files. When KeepFiles is true only existence is verified.
func Remove(ctx context.Context, opts RemoveOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	if opts.KeepFiles {
		logging.Info("xp2p client remove skipping files", "install_dir", installDir)
		return nil
	}

	if err := ensureServiceInactive(ctx, control.RoleClient, "xp2p client service stop"); err != nil {
		return err
	}
	if err := apply.RemoveRoleMarkers(config.ApplyRequestPath(), config.ApplyErrorPath(), apply.RoleClient); err != nil {
		return err
	}

	configDir, err := ResolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	pendingDir, err := config.PendingConfigDir(configDir)
	if err != nil {
		return err
	}
	liveDir, err := config.LiveConfigDir(configDir)
	if err != nil {
		return err
	}
	lkgDir, err := config.LkgConfigDir(configDir)
	if err != nil {
		return err
	}
	paths, err := resolveClientPaths(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	if err := restoreFullTunnel(ctx, paths, false); err != nil {
		return err
	}
	if err := removeNetworkdConfig(opts.TunName); err != nil {
		return err
	}
	if err := openwrt.RemoveTunInterfaceIfManaged(opts.TunName); err != nil {
		return err
	}

	if err := os.RemoveAll(configDir); err != nil {
		return fmt.Errorf("remove client config dir: %w", err)
	}
	if err := os.RemoveAll(pendingDir); err != nil {
		return fmt.Errorf("remove client pending dir: %w", err)
	}
	if err := os.RemoveAll(liveDir); err != nil {
		return fmt.Errorf("remove client live dir: %w", err)
	}
	if err := os.RemoveAll(lkgDir); err != nil {
		return fmt.Errorf("remove client lkg dir: %w", err)
	}

	stateRemoved := false

	configPath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	if err := os.Remove(configPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove client config file: %w", err)
		}
	} else {
		stateRemoved = true
	}

	appliedPath := filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName))
	if err := os.Remove(appliedPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove client applied state: %w", err)
		}
	} else {
		stateRemoved = true
	}

	clientHeartbeatPath := filepath.Join(installDir, layout.ClientHeartbeatStateFileName)
	if err := os.Remove(clientHeartbeatPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove client heartbeat state: %w", err)
	}

	clientStatePath := filepath.Join(installDir, layout.ClientStateFileName)
	legacyStatePath := filepath.Join(installDir, layout.StateFileName)

	if err := os.Remove(clientStatePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove client state file: %w", err)
		}
	} else {
		stateRemoved = true
	}

	if err := installstate.Remove(legacyStatePath, installstate.KindClient); err != nil {
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, installstate.ErrRoleNotInstalled) {
			return fmt.Errorf("remove client state file: %w", err)
		}
	} else {
		stateRemoved = true
	}

	if !opts.KeepFiles {
		if err := removeInstallDirIfUnused(installDir); err != nil {
			return err
		}
	}

	if !stateRemoved && !opts.IgnoreMissing {
		return fmt.Errorf("remove client state file: %w", os.ErrNotExist)
	}

	logging.Info("xp2p client configuration removed", "install_dir", installDir, "config_dir", configDir)
	return nil
}

func deployDesiredConfiguration(ctx context.Context, state installState) error {
	if _, err := ensureClientXrayConfigForce(state.configFile, state.Force); err != nil {
		return err
	}
	if err := extensions.EnsureTemplates(state.configDir); err != nil {
		return err
	}
	resolved, err := resolveEndpointPrimaryAddress(ctx, state.serverHost)
	if err != nil {
		return err
	}
	_, err = applyClientEndpointConfig("", state.configFile, endpointConfig{
		Profile:               state.Profile,
		Protocol:              state.Protocol,
		Transport:             state.Transport,
		Security:              state.Security,
		Flow:                  state.Flow,
		Hostname:              state.serverHost,
		Address:               resolved,
		Port:                  state.serverPort,
		User:                  state.User,
		Password:              state.Password,
		ServerName:            state.serverName,
		ALPN:                  state.ALPN,
		AllowInsecure:         state.AllowInsecure,
		PinnedPeerCertSHA256:  state.PinnedPeerCertSHA256,
		VerifyPeerCertByName:  state.VerifyPeerCertByName,
		AllowInsecureOverride: state.AllowInsecureOverride,
	}, state.Force)
	if err != nil {
		return err
	}
	return nil
}
