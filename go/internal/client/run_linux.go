//go:build linux

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/cli/modemgr"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/health"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/linuxnet"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/openwrt"
	"github.com/NlightN22/xray-p2p/go/internal/preflight"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/xray"
	"github.com/NlightN22/xray-p2p/go/internal/xrayguard"
)

var ensureClientOpenWrtTunInterfaceContextFunc = openwrt.EnsureTunInterfaceContext

// Run launches xray-core using the installed client configuration directory and blocks until the process exits.
func Run(ctx context.Context, opts RunOptions) (retErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	liveConfigDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		return err
	}
	desiredExtensionsDir, err := config.DesiredExtensionsDirForRole(apply.RoleClient)
	if err != nil {
		return err
	}

	appliedStatePath := filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName))
	hasAppliedState := false
	if info, err := os.Stat(appliedStatePath); err == nil {
		if info.IsDir() {
			return fmt.Errorf("%s is a directory, expected client applied state file", appliedStatePath)
		}
		hasAppliedState = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect client applied state %s: %w", appliedStatePath, err)
	}

	if err := seedApplyRequestOnServiceStart(apply.RoleClient, liveConfigDir, desiredExtensionsDir); err != nil {
		return err
	}
	rollback, pendingApplied, request, err := applyPendingIfRequested(apply.RoleClient)
	if err != nil {
		return err
	}
	defer func() {
		if retErr == nil || !pendingApplied || rollback == nil {
			return
		}
		if errors.Is(retErr, context.Canceled) || errors.Is(retErr, context.DeadlineExceeded) {
			return
		}
		if hasAppliedState {
			if restoreErr := rollback.Restore(config.AuditLogPath()); restoreErr != nil {
				logging.Warn("rollback failed after apply", "err", restoreErr)
			} else {
				logging.Warn("rollback completed after apply failure")
			}
		} else {
			logging.Warn("rollback skipped; no applied state yet")
		}
		if request.ID != "" {
			_ = apply.WriteError(config.ApplyErrorPath(), apply.ErrorMarker{
				RequestID: request.ID,
				Role:      apply.RoleClient,
				Reason:    retErr.Error(),
			}, config.AuditLogPath())
		}
	}()
	meta, metaErr := loadLiveRuntimeMeta(liveConfigDir)
	if metaErr != nil {
		return metaErr
	}
	desired := runtimeDesiredToClientInstallState(meta.Desired)
	opts.TunEnabled = meta.TunEnabled
	opts.TunName = meta.TunName
	opts.TunMTU = meta.TunMTU
	opts.TunAddr = meta.TunAddr
	opts.TunMode = meta.TunMode
	opts.DNSServers = meta.DNSServers
	if meta.FullTag != "" {
		opts.FullTunnelTag = meta.FullTag
	}

	xrayPath, err := xray.ResolveBinaryPathForInstallDir(installDir)
	if err != nil {
		return err
	}

	if stat, err := os.Stat(liveConfigDir); err != nil || !stat.IsDir() {
		if err != nil {
			return fmt.Errorf("configuration directory not found at %s: %w", liveConfigDir, err)
		}
		return fmt.Errorf("%s is not a directory", liveConfigDir)
	}

	paths, err := resolveClientPaths(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	paths, err = adjustRunPaths(paths)
	if err != nil {
		return err
	}
	configDir := paths.configDir
	if err := syncXrayAssets(ctx, meta, xrayPath, configDir); err != nil {
		return err
	}
	appliedState, err := loadClientAppliedState(paths.stateFile)
	if err != nil {
		return err
	}

	tunEnabled := opts.TunEnabled
	if err := preflight.CheckTun(ctx, preflight.TunConfig{
		Enabled: tunEnabled,
		Name:    opts.TunName,
		Addr:    opts.TunAddr,
		MTU:     opts.TunMTU,
		Mode:    opts.TunMode,
	}); err != nil {
		return err
	}
	if tunEnabled {
		if err := ensureClientTunSetup(ctx, opts); err != nil {
			return tunSetupErrorWithHint("client run", err)
		}
	} else {
		if err := openwrt.RemoveTunInterfaceIfManagedContext(ctx, opts.TunName); err != nil {
			return err
		}
		if err := linuxnet.RemoveTunInterfaceIfManagedContext(ctx, opts.TunName); err != nil {
			return err
		}
	}

	if !appliedState.matches(desired, tunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr) {
		if err := modemgr.ApplyNatRedirectMode(modeLabel(tunEnabled)); err != nil {
			return err
		}
		if err := saveClientAppliedState(paths.stateFile, desired, tunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr); err != nil {
			return err
		}
	}

	desiredOS := DesiredOSState{
		TunEnabled:        opts.TunEnabled,
		TunName:           opts.TunName,
		TunAddr:           opts.TunAddr,
		TunMTU:            opts.TunMTU,
		TunMode:           opts.TunMode,
		DNSServers:        opts.DNSServers,
		FullTunnelVerbose: opts.FullTunnelVerbose,
		FullTunnelTag:     opts.FullTunnelTag,
		Install:           desired,
	}
	orchestrator := NewOSStateOrchestrator(paths, newLinuxOSStateDriver(paths, opts))

	if strings.TrimSpace(opts.Heartbeat.SocksAddress) == "" {
		addr, err := resolveClientSocksAddress(filepath.Join(configDir, layout.XrayConfigFileName))
		if err != nil {
			logging.Warn("client heartbeat disabled: SOCKS address cannot be resolved", "err", err)
		} else {
			opts.Heartbeat.SocksAddress = addr
		}
	}
	stopHeartbeat := startHeartbeatLoop(ctx, installDir, configDir, opts.Heartbeat)
	defer stopHeartbeat()
	stopSubscriptionSync := startSubscriptionSyncLoop(ctx, installDir, configDir, opts.Heartbeat)
	defer stopSubscriptionSync()
	stopTunRouteRefresh := startTunRouteRefreshLoop(ctx, configDir, opts)
	defer stopTunRouteRefresh()

	reconcileReason := ReconcileReasonServiceRestart
	if pendingApplied && request.ID != "" {
		reconcileReason = ReconcileReasonApplyRequest
	}

	runErr := runXrayWithConfig(
		ctx,
		xrayPath,
		filepath.Join(configDir, layout.XrayConfigFileName),
		configDir,
		nil,
		func() error {
			if _, err := orchestrator.Reconcile(ctx, desiredOS, reconcileReason); err != nil {
				return err
			}
			return nil
		},
		func(readyCtx context.Context) error {
			addr, err := resolveClientSocksAddress(filepath.Join(configDir, layout.XrayConfigFileName))
			if err != nil {
				logging.Warn("client socks health check using defaults", "err", err)
			}
			if opts.TunEnabled {
				refreshClientTunRoutes(readyCtx, opts.TunName, opts.TunAddr, opts.TunMTU, desired.Redirects)
			}
			if err := health.WaitForSocksProxy(readyCtx, addr, socksHealthTimeout, socksHealthInterval); err != nil {
				return err
			}
			runSubscriptionSyncOnce(readyCtx, installDir, configDir, opts.Heartbeat)
			if opts.TunEnabled {
				refreshClientLiveTunRoutes(readyCtx, configDir, opts.TunName, opts.TunAddr, opts.TunMTU)
			}
			return nil
		},
		func(event xrayguard.Event) {
			if err := updateClientRuntimeQuarantine(paths.stateFile, event, opts.FullTunnelTag); err != nil {
				logging.Warn("client runtime quarantine state update failed", "err", err)
			}
		},
	)
	if event, ok := loopProtectionEvent(runErr); ok {
		logging.Warn("client loop protection quarantine delay",
			"reason", event.Reason,
			"fd_delta", event.FDDelta,
			"delay", loopProtectionQuarantineDelay.String(),
		)
		sleepWithContext(ctx, loopProtectionQuarantineDelay)
	}
	return runErr
}

func ensureClientTunSetup(ctx context.Context, opts RunOptions) error {
	if err := ensureClientOpenWrtTunInterfaceContextFunc(ctx, opts.TunName, opts.TunAddr); err != nil {
		return err
	}
	return linuxnet.EnsureTunInterfaceContext(ctx, opts.TunName, opts.TunAddr, opts.TunMTU)
}

func tunSetupErrorWithHint(action string, err error) error {
	return fmt.Errorf("tun setup failed during %s: %w (set XP2P_CLIENT_TUN_ENABLED=false or run \"xp2p client mode proxy\")", action, err)
}

func refreshClientLiveTunRoutes(ctx context.Context, configDir, tunName, tunAddr string, tunMTU int) {
	meta, err := loadLiveRuntimeMeta(configDir)
	if err != nil {
		logging.Warn("client live route metadata refresh failed", "err", err)
		return
	}
	refreshClientTunRoutes(ctx, tunName, tunAddr, tunMTU, runtimeDesiredToClientInstallState(meta.Desired).Redirects)
}

var refreshClientLiveTunRoutesFunc = refreshClientLiveTunRoutes

func startTunRouteRefreshLoop(ctx context.Context, configDir string, opts RunOptions) func() {
	if !opts.TunEnabled {
		return func() {}
	}
	routeCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			refreshClientLiveTunRoutesFunc(routeCtx, configDir, opts.TunName, opts.TunAddr, opts.TunMTU)
			select {
			case <-routeCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func refreshClientTunRoutes(ctx context.Context, tunName, tunAddr string, tunMTU int, redirects []redirect.Rule) {
	deadline := time.Now().Add(20 * time.Second)
	for {
		addrErr := linuxnet.EnsureTunAddressContext(ctx, tunName, tunAddr, tunMTU)
		var routeErr error
		if addrErr == nil {
			routeErr = applyRedirectRoutesContext(ctx, tunName, tunAddr, redirects)
		}
		if addrErr == nil && routeErr == nil {
			return
		}
		if time.Now().After(deadline) {
			if addrErr != nil {
				logging.Warn("client tun address refresh failed after xray start", "err", addrErr)
			}
			if routeErr != nil {
				logging.Warn("client redirect routes refresh failed after xray start", "err", routeErr)
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}
