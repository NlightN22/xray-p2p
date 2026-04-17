//go:build windows

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/extensions"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/service/control"
)

type installState struct {
	InstallOptions
	installDir   string
	binDir       string
	logsDir      string
	configDir    string
	serverPort   int
	serverName   string
	serverRemote string
	configFile   string
	stateFile    string
}

// Install deploys client configuration files.
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
		"server_address", state.serverRemote,
		"server_port", state.serverPort,
		"allow_insecure", state.AllowInsecure,
	)

	if err := os.MkdirAll(state.binDir, 0o755); err != nil {
		return fmt.Errorf("create bin directory: %w", err)
	}
	if err := os.MkdirAll(state.logsDir, 0o755); err != nil {
		return fmt.Errorf("create logs directory: %w", err)
	}
	if err := os.MkdirAll(state.configDir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := ensureClientTunConfig(state.Force, state.TunEnabled, state.TunName, state.TunMTU, state.TunAddr, state.TunMode, state.TunModeSet); err != nil {
		return err
	}

	if err := ensureXrayBinaryPresent(state.binDir); err != nil {
		return err
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

// Remove deletes installation files. When KeepFiles is true only existence is verified.
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

	clientStatePath := filepath.Clean(config.ConfigPath(layout.ClientStateFileName))
	legacyStatePath := filepath.Clean(config.ConfigPath(layout.StateFileName))

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

	if !stateRemoved && !opts.IgnoreMissing {
		return fmt.Errorf("remove client state file: %w", os.ErrNotExist)
	}

	logging.Info("xp2p client configuration removed", "install_dir", installDir, "config_dir", configDir)
	return nil
}

func normalizeInstallOptions(opts InstallOptions) (installState, error) {
	dir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return installState{}, err
	}

	configDir, err := config.DesiredExtensionsDirForRole(apply.RoleClient)
	if err != nil {
		return installState{}, err
	}
	base, err := buildClientInstallBase(dir, configDir, opts)
	if err != nil {
		return installState{}, err
	}

	state := installState{
		InstallOptions: base.installOpts,
		installDir:     base.installDir,
		binDir:         filepath.Join(dir, layout.BinDirName),
		logsDir:        config.LogRoot(),
		configDir:      base.configDir,
		serverPort:     base.portVal,
		serverName:     base.serverName,
		serverRemote:   base.address,
		configFile:     base.configFile,
		stateFile:      base.appliedStateFile,
	}

	return state, nil
}

func resolveInstallDir(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", errors.New("install directory is required")
	}
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve install directory: %w", err)
	}

	if !isSafeInstallDir(abs) {
		return "", fmt.Errorf("install directory %q is not allowed", abs)
	}

	return abs, nil
}

func ResolveConfigDir(base, cfg string) (string, error) {
	cfg = strings.TrimSpace(cfg)
	if cfg == "" {
		cfg = DefaultClientConfigDir
	}
	if filepath.IsAbs(cfg) {
		return cfg, nil
	}
	return filepath.Join(config.ConfigRoot(), cfg), nil
}

func isSafeInstallDir(path string) bool {
	clean := filepath.Clean(path)
	if clean == "." || clean == string(filepath.Separator) {
		return false
	}

	volume := filepath.VolumeName(clean)
	if volume != "" {
		root := volume + string(filepath.Separator)
		if strings.EqualFold(clean, root) {
			return false
		}
	}

	if strings.HasPrefix(clean, `\\`) {
		parts := strings.Split(clean[2:], `\`)
		if len(parts) < 3 {
			return false
		}
	}

	return true
}

func deployDesiredConfiguration(ctx context.Context, state installState) error {
	if _, err := ensureClientXrayConfigForce(state.configFile, state.Force); err != nil {
		return err
	}
	if err := extensions.EnsureTemplates(state.configDir); err != nil {
		return err
	}
	resolved, err := resolveEndpointPrimaryAddress(ctx, state.serverRemote)
	if err != nil {
		return err
	}
	_, err = applyClientEndpointConfig("", state.configFile, endpointConfig{
		Hostname:              state.serverRemote,
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

func ensureXrayBinaryPresent(binDir string) error {
	path := filepath.Join(binDir, "xray.exe")
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xray binary missing at %s (copy xray.exe into this directory before running install)", path)
		}
		return fmt.Errorf("inspect xray binary at %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("expected file at %s, found directory", path)
	}
	return nil
}
