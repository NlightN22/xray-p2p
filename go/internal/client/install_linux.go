//go:build linux

package client

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

//go:embed assets/templates/*
var clientTemplates embed.FS

type installState struct {
	InstallOptions
	installDir string
	configDir  string
	logsDir    string
	serverPort int
	serverName string
	serverHost string
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
	if err := os.MkdirAll(state.logsDir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create log directory: %w", err)
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

	configDir, err := resolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(configDir); err != nil {
		return fmt.Errorf("xp2p: remove client config dir: %w", err)
	}

	clientStatePath := filepath.Join(installDir, layout.ClientStateFileName)
	legacyStatePath := filepath.Join(installDir, layout.StateFileName)
	stateRemoved := false

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

	if !stateRemoved && !opts.IgnoreMissing {
		return fmt.Errorf("xp2p: remove client state file: %w", os.ErrNotExist)
	}

	logging.Info("xp2p client configuration removed", "install_dir", installDir, "config_dir", configDir)
	return nil
}

func normalizeInstallOptions(opts InstallOptions) (installState, error) {
	dir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return installState{}, err
	}

	configDir, err := resolveConfigDir(dir, opts.ConfigDir)
	if err != nil {
		return installState{}, err
	}

	address := strings.TrimSpace(opts.ServerAddress)
	if address == "" {
		return installState{}, errors.New("xp2p: client server address is required")
	}

	portStr := strings.TrimSpace(opts.ServerPort)
	if portStr == "" {
		portStr = "8443"
	}
	portVal, err := strconv.Atoi(portStr)
	if err != nil || portVal <= 0 || portVal > 65535 {
		return installState{}, fmt.Errorf("xp2p: invalid client server port %q", portStr)
	}

	password := strings.TrimSpace(opts.Password)
	if password == "" {
		return installState{}, errors.New("xp2p: client password is required")
	}

	user := strings.TrimSpace(opts.User)
	if user == "" {
		return installState{}, errors.New("xp2p: client user email is required")
	}

	serverName := strings.TrimSpace(opts.ServerName)
	if serverName == "" {
		serverName = address
	}

	logsDir := filepath.Join(layout.UnixLogRoot, "client")

	state := installState{
		InstallOptions: InstallOptions{
			InstallDir:    dir,
			ConfigDir:     opts.ConfigDir,
			ServerAddress: address,
			ServerPort:    portStr,
			User:          user,
			Password:      password,
			ServerName:    serverName,
			AllowInsecure: opts.AllowInsecure,
			Force:         opts.Force,
		},
		installDir: dir,
		configDir:  configDir,
		logsDir:    logsDir,
		serverPort: portVal,
		serverName: serverName,
		serverHost: address,
		stateFile:  filepath.Join(dir, installstate.FileNameForKind(installstate.KindClient)),
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

func resolveConfigDir(base, cfg string) (string, error) {
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
	if err := writeEmbeddedFile("assets/templates/inbounds.json", filepath.Join(state.configDir, "inbounds.json"), 0o644); err != nil {
		return err
	}

	if err := writeEmbeddedFile("assets/templates/logs.json", filepath.Join(state.configDir, "logs.json"), 0o644); err != nil {
		return err
	}

	return applyClientEndpointConfig(state.configDir, state.stateFile, endpointConfig{
		Hostname:      state.serverHost,
		Port:          state.serverPort,
		User:          state.User,
		Password:      state.Password,
		ServerName:    state.serverName,
		AllowInsecure: state.AllowInsecure,
	}, state.Force)
}

func writeEmbeddedFile(name, dest string, perm os.FileMode) error {
	content, err := clientTemplates.ReadFile(name)
	if err != nil {
		return fmt.Errorf("xp2p: load template %s: %w", name, err)
	}
	if err := os.WriteFile(dest, content, perm); err != nil {
		return fmt.Errorf("xp2p: write template %s: %w", dest, err)
	}
	return nil
}
