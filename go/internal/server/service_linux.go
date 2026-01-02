//go:build linux

package server

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/service"
)

// RunService launches the managed server service loop on Linux.
func RunService(ctx context.Context, opts ServiceOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	configDirName := strings.TrimSpace(opts.ConfigDir)
	if configDirName == "" {
		configDirName = DefaultServerConfigDir
	}

	configDirPath, err := resolveConfigDir(installDir, configDirName)
	// resolveConfigDir only defined in linux file? yes.
	if err != nil {
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
	}

	watchPaths := []string{
		filepath.Join(installDir, "bin"),
		configDirPath,
	}
	ignorePaths := []string{
		filepath.Join(installDir, layout.ClientHeartbeatStateFileName),
		filepath.Join(installDir, layout.HeartbeatStateFileName),
		filepath.Join(installDir, layout.ServerHeartbeatStateFileName),
	}

	runnerOpts := service.Options{
		Name:         "server",
		WatchPaths:   watchPaths,
		IgnorePaths:  ignorePaths,
		MaxRestarts:  opts.MaxRestarts,
		RestartDelay: opts.RestartDelay,
	}

	if err := service.Run(ctx, runnerOpts, func(runCtx context.Context) error {
		return Run(runCtx, runOpts)
	}); err != nil {
		return fmt.Errorf("xp2p server service: %w", err)
	}
	return nil
}
