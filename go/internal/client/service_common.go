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

	"github.com/NlightN22/xray-p2p/go/internal/apply"
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

	configDir, err := ResolveConfigDir(installDir, configDirName)
	if err != nil {
		return err
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
		ErrorLogPath:      strings.TrimSpace(opts.XrayLogPath),
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
	configPath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))

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

	if err := ensureLogFile(baseRunOpts.ErrorLogPath); err != nil {
		return err
	}

	logWatcherStop, err := service.StartLogWatcher(ctx, service.LogWatchOptions{
		Name:          "client",
		Paths:         []string{configDir},
		Files:         []string{filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))},
		IgnorePrefix:  []string{apply.ApplyDir(configDir)},
		WatchDebounce: 400 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer logWatcherStop()

	if err := service.Run(ctx, runnerOpts, func(runCtx context.Context) error {
		if _, err := os.Stat(configDir); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				logging.Warn("xp2p client service: configuration directory missing; stopping", "path", configDir)
				return nil
			}
			return fmt.Errorf("xp2p: configuration directory check failed at %s: %w", configDir, err)
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
		return fmt.Errorf("xp2p client service: %w", err)
	}
	return nil
}

func ensureLogFile(path string) error {
	logPath := strings.TrimSpace(path)
	if logPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return fmt.Errorf("xp2p: create log directory %s: %w", filepath.Dir(logPath), err)
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("xp2p: open xray log file %s: %w", logPath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("xp2p: close xray log file %s: %w", logPath, err)
	}
	return nil
}
