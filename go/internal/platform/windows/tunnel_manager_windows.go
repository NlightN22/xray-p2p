//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/ports"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

type TunnelManager struct{}

func NewTunnelManager() *TunnelManager {
	return &TunnelManager{}
}

func (m *TunnelManager) ApplyModeConfig(ctx context.Context, req ports.ModeConfigRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	switch role {
	case "client":
		return m.applyClientModeConfig(ctx, req)
	case "server":
		return m.applyServerModeConfig(ctx, req)
	default:
		return fmt.Errorf("xp2p: unsupported role %q", req.Role)
	}
}

func (m *TunnelManager) WaitServiceRestart(ctx context.Context, req ports.ServiceWaitRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	timeout := parseDuration(req.Timeout, 90*time.Second)
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := waitForApplyRequestClear(waitCtx); err != nil {
		return err
	}

	role, err := mapServiceRole(req.Role)
	if err != nil {
		return err
	}
	ctrl := servicecontrol.Default()
	if err := waitForServiceActive(waitCtx, ctrl, role); err != nil {
		return err
	}
	return nil
}

func (m *TunnelManager) Status(ctx context.Context, req ports.TunStatusRequest) (ports.TunStatus, error) {
	if err := ctx.Err(); err != nil {
		return ports.TunStatus{}, err
	}
	ifIndex, err := resolveTunIndex(ctx, req.Name, req.Addr)
	if err != nil {
		return ports.TunStatus{}, err
	}
	details, err := winnet.InterfaceIPv4Details(ifIndex)
	status := ports.TunStatus{
		IfIndex:    ifIndex,
		IP:         strings.TrimSpace(details.IP),
		Prefix:     int(details.Prefix),
		OperStatus: winnet.InterfaceOperStatusName(details.OperStatus),
		DadState:   winnet.InterfaceDadStateName(details.DadState),
	}
	if err != nil && !errors.Is(err, winnet.ErrTunIPv4Missing) {
		return status, err
	}

	status.Ready = isReadyStatus(status)
	if req.RequireIPv4 && status.IP == "" {
		return status, winnet.ErrTunIPv4Missing
	}
	if req.RequireUp && !strings.EqualFold(status.OperStatus, "up") {
		return status, fmt.Errorf("xp2p: tun interface not up (%s)", status.OperStatus)
	}
	if req.RequireReady && !status.Ready {
		return status, fmt.Errorf("xp2p: tun interface not ready (%s/%s)", status.OperStatus, status.DadState)
	}
	return status, nil
}

func (m *TunnelManager) EnsureReady(ctx context.Context, req ports.TunEnsureRequest) (ports.TunStatus, error) {
	if err := ctx.Err(); err != nil {
		return ports.TunStatus{}, err
	}
	timeout := parseDuration(req.Timeout, 60*time.Second)
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ifIndex, ip, err := winnet.WaitForTunIPv4(waitCtx, req.Name, req.Addr, req.Verbose)
	status := ports.TunStatus{
		IfIndex: ifIndex,
		IP:      strings.TrimSpace(ip),
	}
	if err != nil {
		return status, err
	}
	if details, detailErr := winnet.InterfaceIPv4Details(ifIndex); detailErr == nil {
		status.Prefix = int(details.Prefix)
		status.OperStatus = winnet.InterfaceOperStatusName(details.OperStatus)
		status.DadState = winnet.InterfaceDadStateName(details.DadState)
		status.Ready = isReadyStatus(status)
	}
	return status, nil
}

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
		return ports.RouteResult{}, errors.New("xp2p: install directory is required")
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
		return ports.RouteResult{}, errors.New("xp2p: install directory is required")
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

func (m *TunnelManager) applyClientModeConfig(ctx context.Context, req ports.ModeConfigRequest) error {
	if req.Mode != ports.TunnelModeSplit && req.Mode != ports.TunnelModeFull {
		return fmt.Errorf("xp2p: invalid tun mode %q", req.Mode)
	}
	cfg, err := loadConfigForRole(req.ConfigPath, "client")
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Client.InstallDir) == "" {
		return errors.New("xp2p: install directory is required")
	}
	tunName := firstNonEmpty(req.TunName, cfg.Client.TunName)
	tunAddr := firstNonEmpty(req.TunAddr, cfg.Client.TunAddr)
	tunMTU := req.TunMTU
	if tunMTU <= 0 {
		tunMTU = cfg.Client.TunMTU
	}
	fullTag := strings.TrimSpace(req.FullTag)
	if fullTag == "" {
		fullTag = strings.TrimSpace(cfg.Client.FullTunnelTag)
	}

	if _, err := config.UpdateTunEnabled(req.ConfigPath, "client", true); err != nil {
		return err
	}
	if _, err := config.UpdateTunMode(req.ConfigPath, "client", string(req.Mode)); err != nil {
		return err
	}
	if req.Mode == ports.TunnelModeFull && fullTag != "" {
		if _, err := config.UpdateFullTunnelTag(req.ConfigPath, fullTag); err != nil {
			return err
		}
	}

	if err := client.ApplyModePending(client.ModeOptions{
		InstallDir:    cfg.Client.InstallDir,
		ConfigDir:     cfg.Client.ConfigDir,
		TunEnabled:    true,
		TunName:       tunName,
		TunMTU:        tunMTU,
		TunAddr:       tunAddr,
		TunMode:       string(req.Mode),
		FullTunnelTag: fullTag,
	}); err != nil {
		return err
	}

	reqApply, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), reqApply, config.AuditLogPath())
}

func (m *TunnelManager) applyServerModeConfig(_ context.Context, req ports.ModeConfigRequest) error {
	if _, err := config.UpdateTunEnabled(req.ConfigPath, "server", true); err != nil {
		return err
	}
	reqApply, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), reqApply, config.AuditLogPath())
}

func loadConfigForRole(path string, role string) (config.Config, error) {
	readPath := resolveConfigReadPath(path, role)
	return config.Load(config.Options{
		Path:         readPath,
		AllowInvalid: true,
	})
}

func resolveConfigReadPath(explicit string, role string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	filename := layout.ClientConfigFileName
	if strings.EqualFold(role, "server") {
		filename = layout.ServerConfigFileName
	}
	live := config.ConfigPath(filename)
	if _, err := os.Stat(live); err == nil {
		return live
	}
	return config.PendingConfigPath(filename)
}

func waitForApplyRequestClear(ctx context.Context) error {
	path := config.ApplyRequestPath()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("xp2p: read apply request: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForServiceActive(ctx context.Context, ctrl servicecontrol.Controller, role servicecontrol.Role) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		status, err := ctrl.Status(ctx, role)
		if err != nil {
			if errors.Is(err, servicecontrol.ErrUnsupported) {
				return nil
			}
			return err
		}
		if status.Active {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func mapServiceRole(role string) (servicecontrol.Role, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "client":
		return servicecontrol.RoleClient, nil
	case "server":
		return servicecontrol.RoleServer, nil
	default:
		return "", fmt.Errorf("xp2p: unsupported role %q", role)
	}
}

func resolveTunIndex(ctx context.Context, name, addr string) (int, error) {
	if strings.TrimSpace(addr) != "" {
		ifIndex, err := winnet.InterfaceIndexByIP(addr)
		if err == nil && ifIndex > 0 {
			return ifIndex, nil
		}
		if err != nil && !errors.Is(err, winnet.ErrInterfaceNotFound) {
			return 0, err
		}
	}
	if strings.TrimSpace(name) == "" {
		return 0, winnet.ErrInterfaceNotFound
	}
	ifIndex, _, _, err := winnet.InterfaceByNamePrefix(name)
	if err == nil && ifIndex > 0 {
		return ifIndex, nil
	}
	ifIndex, err = winnet.InterfaceIndexByName(ctx, name)
	if err == nil && ifIndex > 0 {
		return ifIndex, nil
	}
	if errors.Is(err, winnet.ErrInterfaceNotFound) {
		ifIndex, _, _, err = winnet.InterfaceByDescriptionContains([]string{"xray tunnel", "wintun"})
		if err == nil && ifIndex > 0 {
			return ifIndex, nil
		}
	}
	if err != nil {
		return 0, err
	}
	return 0, winnet.ErrInterfaceNotFound
}

func isReadyStatus(status ports.TunStatus) bool {
	return status.IP != "" &&
		strings.EqualFold(status.OperStatus, "up") &&
		strings.EqualFold(status.DadState, "preferred")
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	if parsed, err := time.ParseDuration(trimmed); err == nil {
		return parsed
	}
	if seconds, err := parseInt(trimmed); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func parseInt(value string) (int, error) {
	out, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
