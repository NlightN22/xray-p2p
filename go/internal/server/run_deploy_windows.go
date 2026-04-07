//go:build windows

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/health"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
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

	xrayPath := filepath.Join(installDir, layout.BinDirName, "xray.exe")
	if _, err := os.Stat(xrayPath); err != nil {
		return fmt.Errorf("xp2p: xray binary not found at %s: %w", xrayPath, err)
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
