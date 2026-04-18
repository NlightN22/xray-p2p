package servercmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

var requiredServerArtifacts = []string{layout.XrayConfigFileName, layout.RuntimeMetaFileName}

func ensureServerAssets(ctx context.Context, cfg config.Config, installDir, configDirName, configDirPath string, autoInstall, quiet bool) error {
	present, err := serverAssetsPresent(installDir)
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if handled, err := skipInstallForSystemBinary(installDir); handled {
		return err
	}
	if autoInstall {
		return performInstall(ctx, cfg, installDir, configDirName)
	}
	if quiet {
		return errors.New("installation not found and --quiet supplied (use --auto-install)")
	}
	ok, err := promptYesNoFunc(fmt.Sprintf("Install xray-core into %s?", installDir))
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("installation required to run server")
	}
	return performInstall(ctx, cfg, installDir, configDirName)
}

func performInstall(ctx context.Context, cfg config.Config, installDir, configDirName string) error {
	hostValue, autoDetected, err := determineInstallHost(ctx, "", cfg.Server.Host)
	if err != nil {
		return fmt.Errorf("xp2p server install: detect host: %w", err)
	}
	if autoDetected {
		logging.Info("xp2p server install: detected public host", "host", hostValue)
	}

	opts := server.InstallOptions{
		InstallDir:    installDir,
		ConfigDir:     configDirName,
		Port:          resolveInstallPort(cfg, ""),
		Host:          hostValue,
		TunEnabled:    cfg.Server.TunEnabled,
		TunEnabledSet: true,
		TunName:       cfg.Server.TunName,
		TunMTU:        cfg.Server.TunMTU,
		TunAddr:       cfg.Server.TunAddr,
	}
	if cfg.Server.CertificateStore != "" {
		opts.CertificateStore = cfg.Server.CertificateStore
	}
	if cfg.Server.CertificateFile != "" {
		opts.CertificateFile = cfg.Server.CertificateFile
	}
	if cfg.Server.KeyFile != "" {
		opts.KeyFile = cfg.Server.KeyFile
	}
	return serverInstallFunc(ctx, opts)
}

func serverAssetsPresent(installDir string) (bool, error) {
	binaryName := "xray.exe"
	if runtime.GOOS != "windows" {
		binaryName = "xray"
	}
	binPath := filepath.Join(installDir, layout.BinDirName, binaryName)
	if info, err := os.Stat(binPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", binPath, err)
	} else if info.IsDir() {
		return false, fmt.Errorf("expected file at %s", binPath)
	}

	liveConfigDir, err := config.LiveRoleDir(apply.RoleServer)
	if err != nil {
		return false, err
	}
	if ok, err := configFilesPresent(liveConfigDir, requiredServerArtifacts); err != nil {
		return false, fmt.Errorf("stat %s: %w", liveConfigDir, err)
	} else if ok {
		return true, nil
	}

	if ok, err := desiredInputsPresent(apply.RoleServer); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	return false, nil
}

func configFilesPresent(dir string, names []string) (bool, error) {
	for _, name := range names {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, nil
			}
			return false, fmt.Errorf("stat %s: %w", path, err)
		}
	}
	return true, nil
}

func skipInstallForSystemBinary(installDir string) (bool, error) {
	if runtime.GOOS != "linux" {
		return false, nil
	}
	if filepath.Clean(installDir) != layout.UnixConfigRoot {
		return false, nil
	}

	binPath := filepath.Join(layout.UnixConfigRoot, layout.BinDirName, "xray")
	info, err := os.Stat(binPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, fmt.Errorf("xray binary not found at %s (install the system package or set XP2P_XRAY_BIN)", binPath)
		}
		return true, fmt.Errorf("inspect xray binary at %s: %w", binPath, err)
	}
	if info.IsDir() {
		return true, fmt.Errorf("expected xray binary file at %s", binPath)
	}
	return true, nil
}

func resolveConfigDirPath(installDir, configDir string) (string, error) {
	cfgDir := strings.TrimSpace(configDir)
	if cfgDir == "" {
		cfgDir = server.DefaultServerConfigDir
	}
	if filepath.IsAbs(cfgDir) {
		return cfgDir, nil
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(config.ConfigRoot(), cfgDir), nil
	}
	return filepath.Join(installDir, cfgDir), nil
}
