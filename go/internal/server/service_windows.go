//go:build windows

package server

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/service"
)

// RunService launches the managed server service loop on Windows.
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
	if err != nil {
		return err
	}

	runOpts := RunOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		ErrorLogPath: strings.TrimSpace(opts.XrayLogPath),
	}

	watchPaths := []string{
		installDir,
		configDirPath,
	}
	ignorePaths := []string{
		filepath.Join(installDir, layout.HeartbeatStateFileName),
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
