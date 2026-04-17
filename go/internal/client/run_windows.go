//go:build windows

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/health"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
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

	xrayPath := filepath.Join(installDir, layout.BinDirName, "xray.exe")
	if _, err := os.Stat(xrayPath); err != nil {
		return fmt.Errorf("xray binary not found at %s: %w", xrayPath, err)
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
		var pendingRetry *PendingRetryError
		if errors.As(retErr, &pendingRetry) || errors.Is(retErr, winnet.ErrTunIPv4TentativeTimeout) {
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
	if !appliedState.matches(desired, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr) {
		if err := saveClientAppliedState(paths.stateFile, desired, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr); err != nil {
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
	orchestrator := NewOSStateOrchestrator(paths, newWindowsOSStateDriver(paths, opts))
	reconcileReason := ReconcileReasonServiceRestart
	if pendingApplied && request.ID != "" {
		reconcileReason = ReconcileReasonApplyRequest
	}

	// sendThrough is compiled into xray.json during apply.

	wintunPath := filepath.Join(installDir, layout.BinDirName, "wintun.dll")
	if result, err := winnet.CleanupWintunAdapter(wintunPath, opts.TunName); err != nil {
		logging.Warn("wintun adapter cleanup failed", "interface", opts.TunName, "result", "error", "err", err)
	} else {
		logging.Info("wintun adapter cleanup", "interface", opts.TunName, "result", result)
	}

	stopHeartbeat := startHeartbeatLoop(ctx, installDir, configDir, opts.Heartbeat)
	defer stopHeartbeat()

	configureCmd := func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
		}
	}

	onStart := func() error {
		result, err := orchestrator.Reconcile(ctx, desiredOS, reconcileReason)
		if err != nil {
			_ = updateClientRuntimeState(paths.stateFile, clientRuntimeState{
				LastError: err.Error(),
			})
			return err
		}
		runtime := clientRuntimeState{}
		if opts.TunEnabled {
			runtime.Tun = tunRuntimeState{
				Name:       strings.TrimSpace(opts.TunName),
				IfIndex:    result.Observed.TunIfIndex,
				IPv4:       strings.TrimSpace(result.Observed.TunIPv4),
				OperStatus: strings.TrimSpace(result.Observed.TunOperStatus),
				DadState:   strings.TrimSpace(result.Observed.TunDadState),
				Ready:      result.Observed.TunReady,
			}
			runtime.Routes = routeRuntimeState{
				RedirectApplied: result.RedirectApplied,
				RedirectCount:   result.RedirectCount,
				FullApplied:     result.FullApplied,
				FullBypassCount: result.FullBypassCount,
			}
		}
		if err := updateClientRuntimeState(paths.stateFile, runtime); err != nil {
			logging.Warn("client runtime state update failed", "err", err)
		}
		return nil
	}

	onReady := func(readyCtx context.Context) error {
		addr, err := resolveClientSocksAddress(filepath.Join(configDir, layout.XrayConfigFileName))
		if err != nil {
			logging.Warn("client socks health check using defaults", "err", err)
		}
		readyErr := health.WaitForSocksProxy(readyCtx, addr, socksHealthTimeout, socksHealthInterval)
		runtime := clientRuntimeState{SocksReady: readyErr == nil}
		if readyErr != nil {
			runtime.LastError = readyErr.Error()
		}
		if err := updateClientRuntimeState(paths.stateFile, runtime); err != nil {
			logging.Warn("client runtime state update failed", "err", err)
		}
		return readyErr
	}

	runErr := runXrayWithConfig(
		ctx,
		xrayPath,
		filepath.Join(configDir, layout.XrayConfigFileName),
		installDir,
		configureCmd,
		onStart,
		onReady,
	)
	if runErr != nil {
		var pendingRetry *PendingRetryError
		if errors.As(runErr, &pendingRetry) {
			logging.Warn("tun ready failed; mode change remains pending", "err", runErr)
			sleepWithContext(ctx, pendingRetry.Delay)
			return runErr
		}
		if errors.Is(runErr, winnet.ErrTunIPv4TentativeTimeout) {
			logging.Warn("tun ready failed; mode change remains pending", "err", runErr)
			return runErr
		}
	}
	return runErr
}
