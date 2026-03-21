//go:build linux

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/openwrt"
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
		return fmt.Errorf("xp2p: create config directory: %w", err)
	}
	logRoot := config.LogRoot()
	if err := os.MkdirAll(logRoot, 0o777); err != nil {
		return fmt.Errorf("xp2p: create log root: %w", err)
	}
	if err := os.Chmod(logRoot, 0o777); err != nil {
		logging.Warn("xp2p: chmod log root failed", "path", logRoot, "err", err)
	}
	if err := os.MkdirAll(state.logsDir, 0o777); err != nil {
		return fmt.Errorf("xp2p: create log directory: %w", err)
	}
	if err := os.Chmod(state.logsDir, 0o777); err != nil {
		logging.Warn("xp2p: chmod log directory failed", "path", state.logsDir, "err", err)
	}
	if err := ensureClientTunConfig(state.Force, state.TunEnabled, state.TunName, state.TunMTU, state.TunAddr, state.TunMode, state.TunModeSet); err != nil {
		return err
	}
	if state.TunEnabled {
		if err := openwrt.EnsureTunInterface(state.TunName, state.TunAddr); err != nil {
			return err
		}
	}

	if err := deployConfiguration(state); err != nil {
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

	configDir, err := ResolveConfigDir(installDir, opts.ConfigDir)
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
		return fmt.Errorf("xp2p: remove client config dir: %w", err)
	}

	stateRemoved := false

	configPath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	if err := os.Remove(configPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: remove client config file: %w", err)
		}
	} else {
		stateRemoved = true
	}

	appliedPath := filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName))
	if err := os.Remove(appliedPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: remove client applied state: %w", err)
		}
	} else {
		stateRemoved = true
	}

	clientHeartbeatPath := filepath.Join(installDir, layout.ClientHeartbeatStateFileName)
	if err := os.Remove(clientHeartbeatPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: remove client heartbeat state: %w", err)
	}

	clientStatePath := filepath.Join(installDir, layout.ClientStateFileName)
	legacyStatePath := filepath.Join(installDir, layout.StateFileName)

	if err := os.Remove(clientStatePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: remove client state file: %w", err)
		}
	} else {
		stateRemoved = true
	}

	if err := installstate.Remove(legacyStatePath, installstate.KindClient); err != nil {
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, installstate.ErrRoleNotInstalled) {
			return fmt.Errorf("xp2p: remove client state file: %w", err)
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
		return fmt.Errorf("xp2p: remove client state file: %w", os.ErrNotExist)
	}

	logging.Info("xp2p client configuration removed", "install_dir", installDir, "config_dir", configDir)
	return nil
}

func removeInstallDirIfUnused(installDir string) error {
	clientStatePath := filepath.Join(installDir, installstate.FileNameForKind(installstate.KindClient))
	if installedRole(clientStatePath, installstate.KindClient) {
		return nil
	}
	serverStatePath := filepath.Join(installDir, installstate.FileNameForKind(installstate.KindServer))
	if installedRole(serverStatePath, installstate.KindServer) {
		return nil
	}
	legacyStatePath := filepath.Join(installDir, layout.StateFileName)
	if legacyHasRoles(legacyStatePath) {
		return nil
	}
	if dirHasFiles(filepath.Join(installDir, layout.BinDirName)) {
		return nil
	}
	if err := os.RemoveAll(installDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: remove install dir: %w", err)
	}
	return nil
}

func dirHasFiles(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

func removeNetworkdConfig(tunName string) error {
	name := strings.TrimSpace(tunName)
	if name == "" {
		return nil
	}
	path := filepath.Join("/etc/systemd/network", fmt.Sprintf("90-%s.network", name))
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: remove networkd config: %w", err)
	}
	return nil
}

func installedRole(path string, kind installstate.Kind) bool {
	if _, err := installstate.Read(path, kind); err == nil {
		return true
	} else if errors.Is(err, os.ErrNotExist) || errors.Is(err, installstate.ErrRoleNotInstalled) {
		return false
	}
	return true
}

func legacyHasRoles(path string) bool {
	roles, err := installstate.Roles(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		return true
	}
	return len(roles) > 0
}

func normalizeInstallOptions(opts InstallOptions) (installState, error) {
	dir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return installState{}, err
	}

	configDir, err := ResolveConfigDir(dir, opts.ConfigDir)
	if err != nil {
		return installState{}, err
	}
	base, err := buildClientInstallBase(dir, configDir, opts)
	if err != nil {
		return installState{}, err
	}

	logsDir := filepath.Join(config.LogRoot(), "client")

	state := installState{
		InstallOptions: base.installOpts,
		installDir:     base.installDir,
		configDir:      base.configDir,
		logsDir:        logsDir,
		serverPort:     base.portVal,
		serverName:     base.serverName,
		serverHost:     base.address,
		configFile:     base.configFile,
		stateFile:      base.appliedStateFile,
	}

	return state, nil
}

func resolveInstallDir(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", errors.New("xp2p: install directory is required")
	}
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", fmt.Errorf("xp2p: resolve install directory: %w", err)
		}
		cleaned = abs
	}
	return cleaned, nil
}

func ResolveConfigDir(base, cfg string) (string, error) {
	cfg = strings.TrimSpace(cfg)
	if cfg == "" {
		cfg = layout.ClientConfigDir
	}
	if filepath.IsAbs(cfg) {
		return cfg, nil
	}
	return filepath.Join(base, cfg), nil
}

func deployConfiguration(state installState) error {
	xrayCfg, err := ensureClientXrayConfigForce(state.configFile, state.Force)
	if err != nil {
		return err
	}

	inboundsPath := filepath.Join(state.configDir, "inbounds.json")
	if err := configio.WriteJSON(inboundsPath, buildClientInbounds(xrayCfg, state.TunEnabled, state.TunName, state.TunMTU), configio.WriteOptions{
		AuditPath:         config.AuditLogPath(),
		KeepLastKnownGood: true,
	}); err != nil {
		return err
	}

	if err := configio.WriteJSON(filepath.Join(state.configDir, "logs.json"), buildLogs(xrayCfg.Logs), configio.WriteOptions{
		AuditPath:         config.AuditLogPath(),
		KeepLastKnownGood: true,
	}); err != nil {
		return err
	}

	cfg, err := applyClientEndpointConfig(state.configDir, state.configFile, endpointConfig{
		Hostname:              state.serverHost,
		Port:                  state.serverPort,
		User:                  state.User,
		Password:              state.Password,
		ServerName:            state.serverName,
		AllowInsecure:         state.AllowInsecure,
		PinnedPeerCertSHA256:  state.PinnedPeerCertSHA256,
		VerifyPeerCertByName:  state.VerifyPeerCertByName,
		AllowInsecureOverride: state.AllowInsecureOverride,
	}, state.Force)
	if err != nil {
		return err
	}
	return saveClientAppliedState(state.stateFile, cfg, state.TunEnabled, state.TunName, state.TunMTU, state.TunAddr)
}
