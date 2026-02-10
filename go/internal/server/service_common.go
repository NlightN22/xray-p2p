package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	if _, err := resolveConfigDir(installDir, configDirName); err != nil {
		return err
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
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		ErrorLogPath: strings.TrimSpace(opts.XrayLogPath),
		TunEnabled:   opts.TunEnabled,
		TunName:      opts.TunName,
		TunMTU:       opts.TunMTU,
		TunAddr:      opts.TunAddr,
	}

	watchPaths := []string{
		filepath.Join(installDir, "bin"),
	}
	watchFiles := []string{
		filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)),
	}
	ignorePaths := []string{
		filepath.Join(installDir, layout.ClientHeartbeatStateFileName),
		filepath.Join(installDir, layout.HeartbeatStateFileName),
		filepath.Join(installDir, layout.ServerHeartbeatStateFileName),
	}

	runnerOpts := service.Options{
		Name:          "server",
		WatchPaths:    watchPaths,
		WatchFiles:    watchFiles,
		WatchDebounce: 400 * time.Millisecond,
		IgnorePaths:   ignorePaths,
		MaxRestarts:   opts.MaxRestarts,
		RestartDelay:  opts.RestartDelay,
	}

	if err := ensureLogFile(runOpts.ErrorLogPath); err != nil {
		return err
	}

	if err := service.Run(ctx, runnerOpts, func(runCtx context.Context) error {
		return Run(runCtx, runOpts)
	}); err != nil {
		return fmt.Errorf("xp2p server service: %w", err)
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
