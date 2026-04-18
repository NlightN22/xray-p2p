//go:build windows

package windows

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/ports"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func (m *TunnelManager) ApplySplitRoutes(ctx context.Context, req ports.SplitRouteRequest) (ports.RouteResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.RouteResult{}, err
	}
	if req.AssignIP {
		waitCtx := ctx
		if wait := parseDuration(req.AssignWait, 10*time.Second); wait > 0 {
			var cancel context.CancelFunc
			waitCtx, cancel = context.WithTimeout(ctx, wait)
			defer cancel()
		}
		if _, _, err := winnet.EnsureTunIPv4(waitCtx, req.Name, req.Addr, req.Verbose); err != nil {
			return ports.RouteResult{}, err
		}
	}

	tunAddr := req.Addr
	if !req.AssignIP {
		tunAddr = ""
	}
	err := winnet.SyncRedirectRoutes(req.Name, tunAddr, req.CIDRs)
	if err != nil {
		if errors.Is(err, winnet.ErrInterfaceMissing) || errors.Is(err, winnet.ErrTunIPv4Missing) {
			logging.Warn("redirect route setup skipped", "interface", strings.TrimSpace(req.Name), "err", err)
			return ports.RouteResult{Applied: false}, nil
		}
		return ports.RouteResult{Applied: false}, err
	}
	status, _ := m.Status(ctx, ports.TunStatusRequest{Name: req.Name, Addr: req.Addr})
	return ports.RouteResult{Applied: true, Status: status}, nil
}

func (m *TunnelManager) ApplyFullRoutes(ctx context.Context, req ports.FullRouteRequest) (ports.RouteResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.RouteResult{}, err
	}
	cfg, err := loadConfigForRole("", "client")
	if err != nil {
		return ports.RouteResult{}, err
	}
	if strings.TrimSpace(cfg.Client.InstallDir) == "" {
		return ports.RouteResult{}, errors.New("install directory is required")
	}
	tunName := firstNonEmpty(req.Name, cfg.Client.TunName)
	tunAddr := firstNonEmpty(req.Addr, cfg.Client.TunAddr)

	opts := client.RunOptions{
		InstallDir:        cfg.Client.InstallDir,
		ConfigDir:         cfg.Client.ConfigDir,
		TunEnabled:        true,
		TunName:           tunName,
		TunMTU:            cfg.Client.TunMTU,
		TunAddr:           tunAddr,
		TunMode:           "full",
		DNSServers:        cfg.Client.DNSServers,
		FullTunnelVerbose: req.Verbose || cfg.Client.FullTunnelVerbose,
		FullTunnelTag:     cfg.Client.FullTunnelTag,
	}
	applied, err := client.ApplyFullTunnelRoutes(ctx, opts, req.BypassTargets, req.ForceDefault)
	status, _ := m.Status(ctx, ports.TunStatusRequest{Name: tunName, Addr: tunAddr})
	return ports.RouteResult{Applied: applied, Status: status}, err
}

func (m *TunnelManager) RestoreRoutes(ctx context.Context, req ports.RouteRestoreRequest) (ports.RouteResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.RouteResult{}, err
	}
	cfg, err := loadConfigForRole("", "client")
	if err != nil {
		return ports.RouteResult{}, err
	}
	if strings.TrimSpace(cfg.Client.InstallDir) == "" {
		return ports.RouteResult{}, errors.New("install directory is required")
	}
	if err := client.RestoreFullTunnelRoutes(ctx, cfg.Client.InstallDir, cfg.Client.ConfigDir, req.Verbose); err != nil {
		return ports.RouteResult{}, err
	}
	tunName := firstNonEmpty(req.Name, cfg.Client.TunName)
	tunAddr := firstNonEmpty(req.Addr, cfg.Client.TunAddr)
	status, _ := m.Status(ctx, ports.TunStatusRequest{Name: tunName, Addr: tunAddr})
	return ports.RouteResult{Applied: true, Status: status}, nil
}

func (m *TunnelManager) CleanupTun(ctx context.Context, req ports.TunCleanupRequest) (ports.TunCleanupResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.TunCleanupResult{}, err
	}
	result, err := winnet.CleanupWintunAdapter(req.WintunPath, req.Name)
	if err != nil {
		return ports.TunCleanupResult{
			Result: string(result),
			Errors: []string{err.Error()},
		}, err
	}
	return ports.TunCleanupResult{Result: string(result)}, nil
}
