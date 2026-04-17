//go:build linux

package client

import "context"

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

func (d *linuxOSStateDriver) EnsureSplit(ctx context.Context, desired DesiredOSState) (bool, error) {
	if !desired.TunEnabled {
		return false, nil
	}
	if err := applyRedirectRoutes(desired.TunName, desired.TunAddr, desired.Install.Redirects); err != nil {
		return false, err
	}
	return true, nil
}

func (d *linuxOSStateDriver) RemoveSplit(ctx context.Context, desired DesiredOSState) error {
	return removeRedirectRoutes(desired.TunName, desired.TunAddr, desired.Install.Redirects)
}

func (d *linuxOSStateDriver) EnsureFull(ctx context.Context, desired DesiredOSState) (bool, error) {
	return ensureFullTunnel(ctx, d.paths, d.opts, desired.Install)
}

func (d *linuxOSStateDriver) RollbackFull(ctx context.Context, desired DesiredOSState) error {
	return restoreFullTunnel(ctx, d.paths, desired.FullTunnelVerbose)
}

