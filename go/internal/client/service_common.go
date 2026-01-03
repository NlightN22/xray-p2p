package client

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

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

	configDirPath, err := resolveConfigDir(installDir, configDirName)
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

	runOpts := RunOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		ErrorLogPath: strings.TrimSpace(opts.XrayLogPath),
		Heartbeat:    opts.Heartbeat,
	}

	watchPaths := []string{
		filepath.Join(installDir, "bin"),
		configDirPath,
	}
	ignorePaths := []string{
		filepath.Join(installDir, layout.HeartbeatStateFileName),
		filepath.Join(installDir, layout.ClientHeartbeatStateFileName),
		filepath.Join(installDir, layout.ServerHeartbeatStateFileName),
	}

	runnerOpts := service.Options{
		Name:         "client",
		WatchPaths:   watchPaths,
		IgnorePaths:  ignorePaths,
		MaxRestarts:  opts.MaxRestarts,
		RestartDelay: opts.RestartDelay,
	}

	if err := service.Run(ctx, runnerOpts, func(runCtx context.Context) error {
		return Run(runCtx, runOpts)
	}); err != nil {
		return fmt.Errorf("xp2p client service: %w", err)
	}
	return nil
}
