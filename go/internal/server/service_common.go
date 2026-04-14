package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/service"
)

func runServerServiceCommon(ctx context.Context, opts ServiceOptions) error {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	configDirName := strings.TrimSpace(opts.ConfigDir)
	if configDirName == "" {
		configDirName = DefaultServerConfigDir
	}

	desiredConfigDir, err := ResolveConfigDir(installDir, configDirName)
	if err != nil {
		return err
	}
	pendingConfigDir, err := config.PendingConfigDir(desiredConfigDir)
	if err != nil {
		return err
	}
	liveConfigDir, err := config.LiveConfigDir(desiredConfigDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(desiredConfigDir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create config directory: %w", err)
	}
	if err := os.MkdirAll(pendingConfigDir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create pending config directory: %w", err)
	}

	var diagCancel context.CancelFunc
	if port := strings.TrimSpace(opts.DiagPort); port != "" {
		bgCtx, cancel := context.WithCancel(ctx)
		if err := StartBackground(bgCtx, Options{Port: port, InstallDir: installDir}); err != nil {
			cancel()
			logging.Warn("xp2p server diagnostics: failed to start responders", "port", port, "err", err)
		} else {
			diagCancel = cancel
		}
	}
	defer func() {
		if diagCancel != nil {
			diagCancel()
		}
	}()

	runOpts := RunOptions{
		InstallDir: installDir,
		ConfigDir:  configDirName,
		TunEnabled: opts.TunEnabled,
		TunName:    opts.TunName,
		TunMTU:     opts.TunMTU,
		TunAddr:    opts.TunAddr,
	}

	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	ignorePaths := []string{
		filepath.Join(stateRoot, layout.ClientHeartbeatStateFileName),
		filepath.Join(stateRoot, layout.HeartbeatStateFileName),
		filepath.Join(stateRoot, layout.ServerHeartbeatStateFileName),
	}

	runnerOpts := service.Options{
		Name:               "server",
		WatchFiles:         []string{config.ApplyRequestPath()},
		WatchDebounce:      400 * time.Millisecond,
		IgnorePaths:        ignorePaths,
		MaxRestarts:        opts.MaxRestarts,
		RestartDelay:       opts.RestartDelay,
		MaxWatchRestarts:   3,
		WatchRestartWindow: 30 * time.Second,
	}

	logWatcherStop, err := service.StartLogWatcher(ctx, service.LogWatchOptions{
		Name:          "server",
		Paths:         []string{desiredConfigDir},
		Files:         []string{filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))},
		IgnorePrefix:  []string{config.StateRoot()},
		WatchDebounce: 400 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer logWatcherStop()

	if err := service.Run(ctx, runnerOpts, func(runCtx context.Context) error {
		hasConfig, err := hasServerConfig(liveConfigDir, pendingConfigDir)
		if err != nil {
			return err
		}
		if !hasConfig {
			logging.Info("xp2p server service: no config available; stopping",
				"config_dir", liveConfigDir,
				"config_file", filepath.Clean(config.LiveConfigPath(layout.ServerConfigFileName)),
			)
			return nil
		}
		return Run(runCtx, runOpts)
	}); err != nil {
		return fmt.Errorf("xp2p server service: %w", err)
	}
	return nil
}

func hasServerConfig(liveConfigDir, pendingConfigDir string) (bool, error) {
	liveConfig := filepath.Clean(config.LiveConfigPath(layout.ServerConfigFileName))
	if _, err := os.Stat(liveConfig); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("xp2p: stat %s: %w", liveConfig, err)
	}

	pendingConfig := filepath.Clean(config.PendingConfigPath(layout.ServerConfigFileName))
	if pendingConfig != "" {
		if _, err := os.Stat(pendingConfig); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("xp2p: stat %s: %w", pendingConfig, err)
		}
	}

	if ok, err := configFilesPresent(liveConfigDir, runRequiredConfigFiles); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}

	return configFilesPresent(pendingConfigDir, runRequiredConfigFiles)
}
