//go:build windows

package client

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type windowsOSStateDriver struct {
	paths clientPaths
	opts  RunOptions
}

func newWindowsOSStateDriver(paths clientPaths, opts RunOptions) osStateDriver {
	return &windowsOSStateDriver{
		paths: paths,
		opts:  opts,
	}
}

func (d *windowsOSStateDriver) EnsureSplit(ctx context.Context, desired DesiredOSState) (bool, error) {
	if !desired.TunEnabled {
		return false, nil
	}
	if windowsRoutesDisabled {
		logging.Info("windows route apply disabled; skipping redirect routes")
		return false, nil
	}
	if err := applyRedirectRoutes(desired.TunName, desired.TunAddr, desired.Install.Redirects); err != nil {
		return false, err
	}
	return true, nil
}

func (d *windowsOSStateDriver) RemoveSplit(ctx context.Context, desired DesiredOSState) error {
	if windowsRoutesDisabled {
		return nil
	}
	return removeRedirectRoutes(desired.TunName, desired.TunAddr, desired.Install.Redirects)
}

func (d *windowsOSStateDriver) EnsureFull(ctx context.Context, desired DesiredOSState) (bool, error) {
	if windowsRoutesDisabled {
		logging.Warn("full-tunnel route apply disabled on windows")
		return false, nil
	}
	return ensureFullTunnel(ctx, d.paths, d.opts, desired.Install)
}

func (d *windowsOSStateDriver) RollbackFull(ctx context.Context, desired DesiredOSState) error {
	if windowsRoutesDisabled {
		return nil
	}
	return restoreFullTunnel(ctx, d.paths, desired.FullTunnelVerbose)
}

