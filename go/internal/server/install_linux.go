//go:build linux

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
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
	portValue  int
	selfSigned bool
	stateFile  string
	certSource string
}

// Install deploys server configuration files on Linux/OpenWrt hosts.
func Install(ctx context.Context, opts InstallOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	state, err := normalizeInstallOptions(opts)
	if err != nil {
		return err
	}

	if !state.Force {
		if occupied, reason, err := serverArtifactsPresent(state); err != nil {
			return err
		} else if occupied {
			return fmt.Errorf("server files already present (%s) (use --force to overwrite)", reason)
		}
	}

	logging.Info("xp2p server install starting",
		"install_dir", state.installDir,
		"config_dir", state.configDir,
		"port", state.portValue,
		"host", state.Host,
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
	if _, err := config.EnsureTunSettings("", "server", state.TunEnabled, state.TunName, state.TunMTU, state.TunAddr); err != nil {
		if state.Force && errors.Is(err, config.ErrConfigParse) {
			configPath := config.PendingConfigPath(layout.ServerConfigFileName)
			if removeErr := os.Remove(configPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return removeErr
			}
			if _, retryErr := config.EnsureTunSettings("", "server", state.TunEnabled, state.TunName, state.TunMTU, state.TunAddr); retryErr != nil {
				return retryErr
			}
		} else {
			return err
		}
	}
	if state.TunEnabled {
		if err := openwrt.EnsureTunInterface(state.TunName, state.TunAddr); err != nil {
			return err
		}
	}

	if err := deployDesiredConfiguration(state); err != nil {
		return err
	}
	if err := installstate.Write(state.stateFile, installstate.KindServer); err != nil {
		return fmt.Errorf("write server state: %w", err)
	}
	req, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		return err
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		return err
	}

	logging.Info("xp2p server install completed", "install_dir", state.installDir)
	return nil
}

// Remove deletes server configuration files. When KeepFiles is true only existence is verified.
func Remove(ctx context.Context, opts RemoveOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	if opts.KeepFiles {
		logging.Info("xp2p server remove skipping files", "install_dir", installDir)
		return nil
	}

	if err := ensureServiceInactive(ctx, control.RoleServer, "xp2p server service stop"); err != nil {
		return err
	}
	if err := apply.RemoveRoleMarkers(config.ApplyRequestPath(), config.ApplyErrorPath(), apply.RoleServer); err != nil {
		return err
	}

	configDir, err := config.DesiredExtensionsDirForRole(apply.RoleServer)
	if err != nil {
		return err
	}
	liveDir, err := config.LiveRoleDir(apply.RoleServer)
	if err != nil {
		return err
	}
	lkgDir, err := config.LkgRoleDir(apply.RoleServer)
	if err != nil {
		return err
	}
	if err := removeNetworkdConfig(opts.TunName); err != nil {
		return err
	}
	if err := openwrt.RemoveTunInterfaceIfManaged(opts.TunName); err != nil {
		return err
	}

	if err := os.RemoveAll(configDir); err != nil {
		return fmt.Errorf("remove server config dir: %w", err)
	}
	if err := os.RemoveAll(liveDir); err != nil {
		return fmt.Errorf("remove server live dir: %w", err)
	}
	if err := os.RemoveAll(lkgDir); err != nil {
		return fmt.Errorf("remove server lkg dir: %w", err)
	}

	configPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	if err := os.Remove(configPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove server config file: %w", err)
		}
	}

	appliedPath := filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName))
	if err := os.Remove(appliedPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove server applied state: %w", err)
		}
	}

	serverHeartbeatPath := filepath.Join(installDir, layout.ServerHeartbeatStateFileName)
	if err := os.Remove(serverHeartbeatPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove server heartbeat state: %w", err)
	}

	statePath := filepath.Join(installDir, installstate.FileNameForKind(installstate.KindServer))
	if err := installstate.Remove(statePath, installstate.KindServer); err != nil {
		if !(opts.IgnoreMissing && (errors.Is(err, os.ErrNotExist) || errors.Is(err, installstate.ErrRoleNotInstalled))) {
			return fmt.Errorf("remove server state file: %w", err)
		}
	}
	legacyStatePath := filepath.Join(installDir, layout.StateFileName)
	if err := installstate.Remove(legacyStatePath, installstate.KindServer); err != nil {
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, installstate.ErrRoleNotInstalled) {
			return fmt.Errorf("remove legacy server state: %w", err)
		}
	}

	if !opts.KeepFiles {
		if err := removeInstallDirIfUnused(installDir); err != nil {
			return err
		}
	}

	logging.Info("xp2p server configuration removed", "install_dir", installDir, "config_dir", configDir)
	return nil
}
