//go:build linux

package server

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/NlightN22/xray-p2p/go/internal/health"
	"github.com/NlightN22/xray-p2p/go/internal/xray"
)

// RunDeploy launches xray-core for deploy validation without applying pending config.
func RunDeploy(ctx context.Context, opts DeployRunOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	liveConfigDir, err := ResolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	configDir, configFile, err := adjustRunPaths(liveConfigDir)
	if err != nil {
		return err
	}

	if stat, err := os.Stat(configDir); err != nil || !stat.IsDir() {
		if err != nil {
			return fmt.Errorf("xp2p: configuration directory not found at %s: %w", configDir, err)
		}
		return fmt.Errorf("xp2p: %s is not a directory", configDir)
	}

	xrayPath, err := xray.ResolveBinaryPath()
	if err != nil {
		return err
	}

	runErr := runXrayWithConfig(
		ctx,
		xrayPath,
		configDir,
		configDir,
		nil,
		func(readyCtx context.Context) error {
			addr, err := resolveServerSocksAddress(configFile)
			if err != nil {
				return err
			}
			return health.WaitForSocksProxy(readyCtx, addr, socksHealthTimeout, socksHealthInterval)
		},
	)
	if runErr != nil && errors.Is(runErr, context.Canceled) {
		return nil
	}
	return runErr
}
