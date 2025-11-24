//go:build linux

package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/service"
)

// RunService launches the managed client service loop on Linux.
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
		configDirName = DefaultClientConfigDir
	}

	configDirPath, err := resolveConfigDir(installDir, configDirName)
	if err != nil {
		return err
	}

	runOpts := RunOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		ErrorLogPath: strings.TrimSpace(opts.XrayLogPath),
		Heartbeat:    opts.Heartbeat,
	}

	watchPaths := []string{
		installDir,
		configDirPath,
	}

	runnerOpts := service.Options{
		Name:         "client",
		WatchPaths:   watchPaths,
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
