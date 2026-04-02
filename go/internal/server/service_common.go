package server

import (
	"context"
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

	configDir, err := ResolveConfigDir(installDir, configDirName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create config directory: %w", err)
	}
	if err := os.MkdirAll(apply.PendingDir(configDir), 0o755); err != nil {
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
		Paths:         []string{configDir},
		Files:         []string{filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))},
		IgnorePrefix:  []string{apply.ApplyDir(configDir)},
		WatchDebounce: 400 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer logWatcherStop()

	if err := service.Run(ctx, runnerOpts, func(runCtx context.Context) error {
		return Run(runCtx, runOpts)
	}); err != nil {
		return fmt.Errorf("xp2p server service: %w", err)
	}
	return nil
}
