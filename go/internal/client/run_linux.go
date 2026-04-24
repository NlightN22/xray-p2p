//go:build linux

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/cli/modemgr"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/health"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/linuxnet"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/openwrt"
	"github.com/NlightN22/xray-p2p/go/internal/preflight"
	"github.com/NlightN22/xray-p2p/go/internal/xray"
)

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
		if err := openwrt.EnsureTunInterface(opts.TunName, opts.TunAddr); err != nil {
			return tunSetupErrorWithHint("client run", err)
		}
		if err := linuxnet.EnsureTunInterface(opts.TunName, opts.TunAddr, opts.TunMTU); err != nil {
			return tunSetupErrorWithHint("client run", err)
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

	stopHeartbeat := startHeartbeatLoop(ctx, installDir, configDir, opts.Heartbeat)
	defer stopHeartbeat()

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
			if err := health.WaitForSocksProxy(readyCtx, addr, socksHealthTimeout, socksHealthInterval); err != nil {
				return err
			}
			return nil
		},
	)
	return runErr
}

func tunSetupErrorWithHint(action string, err error) error {
	return fmt.Errorf("tun setup failed during %s: %w (set XP2P_CLIENT_TUN_ENABLED=false or run \"xp2p client mode proxy\")", action, err)
}
