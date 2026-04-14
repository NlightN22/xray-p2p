package client

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
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/service"
)

func runClientServiceCommon(ctx context.Context, opts ServiceOptions) error {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	configDirName := strings.TrimSpace(opts.ConfigDir)
	if configDirName == "" {
		configDirName = DefaultClientConfigDir
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
		if err := server.StartBackground(bgCtx, server.Options{Port: port, InstallDir: installDir}); err != nil {
			cancel()
			logging.Warn("xp2p client diagnostics: failed to start responders", "port", port, "err", err)
		} else {
			diagCancel = cancel
		}
	}
	defer func() {
		if diagCancel != nil {
			diagCancel()
		}
	}()

	baseRunOpts := RunOptions{
		InstallDir:        installDir,
		ConfigDir:         configDirName,
		Heartbeat:         opts.Heartbeat,
		TunEnabled:        opts.TunEnabled,
		TunName:           opts.TunName,
		TunMTU:            opts.TunMTU,
		TunAddr:           opts.TunAddr,
		TunMode:           opts.TunMode,
		DNSServers:        opts.DNSServers,
		FullTunnelVerbose: opts.FullTunnelVerbose,
		FullTunnelTag:     opts.FullTunnelTag,
	}
	configPath := filepath.Clean(config.LiveConfigPath(layout.ClientConfigFileName))

	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	ignorePaths := []string{
		filepath.Join(stateRoot, layout.HeartbeatStateFileName),
		filepath.Join(stateRoot, layout.ClientHeartbeatStateFileName),
		filepath.Join(stateRoot, layout.ServerHeartbeatStateFileName),
	}

	runnerOpts := service.Options{
		Name:               "client",
		WatchFiles:         []string{config.ApplyRequestPath()},
		WatchDebounce:      400 * time.Millisecond,
		IgnorePaths:        ignorePaths,
		MaxRestarts:        opts.MaxRestarts,
		RestartDelay:       opts.RestartDelay,
		MaxWatchRestarts:   3,
		WatchRestartWindow: 30 * time.Second,
	}

	logWatcherStop, err := service.StartLogWatcher(ctx, service.LogWatchOptions{
		Name:          "client",
		Paths:         []string{desiredConfigDir},
		Files:         []string{filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))},
		IgnorePrefix:  []string{config.StateRoot()},
		WatchDebounce: 400 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer logWatcherStop()

	if err := service.Run(ctx, runnerOpts, func(runCtx context.Context) error {
		hasConfig, err := hasClientConfig(liveConfigDir, pendingConfigDir)
		if err != nil {
			return err
		}
		if !hasConfig {
			logging.Info("xp2p client service: no config available; stopping",
				"config_dir", liveConfigDir,
				"config_file", configPath,
			)
			return nil
		}
		runOpts := baseRunOpts
		if cfg, err := config.Load(config.Options{Path: configPath}); err != nil {
			logging.Warn("xp2p client service: failed to reload config", "err", err)
		} else {
			runOpts.TunEnabled = cfg.Client.TunEnabled
			runOpts.TunName = cfg.Client.TunName
			runOpts.TunMTU = cfg.Client.TunMTU
			runOpts.TunAddr = cfg.Client.TunAddr
			runOpts.TunMode = cfg.Client.TunMode
			runOpts.DNSServers = cfg.Client.DNSServers
			runOpts.FullTunnelVerbose = runOpts.FullTunnelVerbose || cfg.Client.FullTunnelVerbose
			runOpts.FullTunnelTag = cfg.Client.FullTunnelTag
		}
		return Run(runCtx, runOpts)
	}); err != nil {
		restoreFullTunnelOnStop(installDir, configDirName)
		return fmt.Errorf("xp2p client service: %w", err)
	}
	restoreFullTunnelOnStop(installDir, configDirName)
	return nil
}

func hasClientConfig(liveConfigDir, pendingConfigDir string) (bool, error) {
	liveConfig := filepath.Clean(config.LiveConfigPath(layout.ClientConfigFileName))
	if _, err := os.Stat(liveConfig); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("xp2p: stat %s: %w", liveConfig, err)
	}

	if ok, err := configFilesPresent(liveConfigDir, runRequiredConfigFiles); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}

	if _, err := os.Stat(config.ApplyRequestPath()); err == nil {
		pendingConfig := filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName))
		if pendingConfig != "" {
			if _, err := os.Stat(pendingConfig); err == nil {
				return true, nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return false, fmt.Errorf("xp2p: stat %s: %w", pendingConfig, err)
			}
		}
		return configFilesPresent(pendingConfigDir, runRequiredConfigFiles)
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("xp2p: stat %s: %w", config.ApplyRequestPath(), err)
	}

	return false, nil
}
