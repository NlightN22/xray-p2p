//go:build windows

package client

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
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

func (d *windowsOSStateDriver) EnsureTunReady(ctx context.Context, desired DesiredOSState) (ObservedOSState, error) {
	if !desired.TunEnabled {
		return ObservedOSState{}, nil
	}
	if windowsRoutesDisabled {
		return ObservedOSState{}, nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	ifIndex, ip, err := winnet.EnsureTunIPv4(waitCtx, desired.TunName, desired.TunAddr, desired.FullTunnelVerbose)
	if err != nil {
		switch {
		case errors.Is(err, winnet.ErrTunIPv4TentativeTimeout):
			return ObservedOSState{}, &OSPendingError{Reason: "tun_ipv4_tentative", Err: err}
		case errors.Is(err, winnet.ErrInterfaceNotFound):
			return ObservedOSState{}, &OSPendingError{Reason: "adapter_not_found", Err: err}
		case errors.Is(err, winnet.ErrTunIPv4Missing):
			return ObservedOSState{}, &OSPendingError{Reason: "tun_ipv4_missing", Err: err}
		default:
			return ObservedOSState{}, err
		}
	}

	go winnet.DisableIPv6BindingWithRetry(ctx, desired.TunName)

	observed := ObservedOSState{
		TunIfIndex: ifIndex,
		TunIPv4:    strings.TrimSpace(ip),
	}
	if details, detailErr := winnet.InterfaceIPv4Details(ifIndex); detailErr == nil {
		observed.TunOperStatus = winnet.InterfaceOperStatusName(details.OperStatus)
		observed.TunDadState = winnet.InterfaceDadStateName(details.DadState)
		observed.TunReady = details.IP != "" &&
			strings.EqualFold(observed.TunOperStatus, "up") &&
			strings.EqualFold(observed.TunDadState, "preferred")
	} else {
		observed.TunReady = observed.TunIPv4 != ""
	}
	if !observed.TunReady {
		return observed, &OSPendingError{Reason: "adapter_not_ready", Err: errors.New("tun adapter not ready")}
	}
	return observed, nil
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
