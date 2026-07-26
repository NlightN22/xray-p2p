//go:build linux

package client

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/linuxnet"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type linuxOSStateDriver struct {
	paths clientPaths
	opts  RunOptions
}

func newLinuxOSStateDriver(paths clientPaths, opts RunOptions) osStateDriver {
	return &linuxOSStateDriver{
		paths: paths,
		opts:  opts,
	}
}

func (d *linuxOSStateDriver) EnsureTunReady(ctx context.Context, desired DesiredOSState) (ObservedOSState, error) {
	if !desired.TunEnabled {
		return ObservedOSState{}, nil
	}
	if err := linuxnet.RemoveTunInterfacesExceptContext(ctx, desired.TunName, "xp2pc", "xp2ps"); err != nil {
		return ObservedOSState{}, err
	}
	if err := linuxnet.EnsureTunAddressContext(ctx, desired.TunName, desired.TunAddr, desired.TunMTU); err != nil {
		logging.Warn("tun address setup failed", "interface", desired.TunName, "err", err)
	}
	return ObservedOSState{TunReady: true}, nil
}

func (d *linuxOSStateDriver) EnsureSplit(ctx context.Context, desired DesiredOSState) (bool, error) {
	if !desired.TunEnabled {
		return false, nil
	}
	if err := applyRedirectRoutesContext(ctx, desired.TunName, desired.TunAddr, desired.Install.Redirects); err != nil {
		return false, err
	}
	return true, nil
}

func (d *linuxOSStateDriver) RemoveSplit(ctx context.Context, desired DesiredOSState) error {
	return removeRedirectRoutesContext(ctx, desired.TunName, desired.TunAddr, desired.Install.Redirects)
}

func (d *linuxOSStateDriver) EnsureFull(ctx context.Context, desired DesiredOSState) (bool, error) {
	return ensureFullTunnel(ctx, d.paths, d.opts, desired.Install)
}

func (d *linuxOSStateDriver) RollbackFull(ctx context.Context, desired DesiredOSState) error {
	return restoreFullTunnel(ctx, d.paths, desired.FullTunnelVerbose)
}
