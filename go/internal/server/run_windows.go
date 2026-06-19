//go:build windows

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/health"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/preflight"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

// Run launches xray-core using the installed configuration directory and blocks until the process exits.
func Run(ctx context.Context, opts RunOptions) (retErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	liveConfigDir, err := config.LiveRoleDir(apply.RoleServer)
	if err != nil {
		return err
	}
	desiredExtensionsDir, err := config.DesiredExtensionsDirForRole(apply.RoleServer)
	if err != nil {
		return err
	}

	appliedStatePath := filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName))
	hasAppliedState := false
	if info, err := os.Stat(appliedStatePath); err == nil {
		if info.IsDir() {
			return fmt.Errorf("%s is a directory, expected server applied state file", appliedStatePath)
		}
		hasAppliedState = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect server applied state %s: %w", appliedStatePath, err)
	}

	xrayPath := filepath.Join(installDir, layout.BinDirName, "xray.exe")
	if _, err := os.Stat(xrayPath); err != nil {
		return fmt.Errorf("xray binary not found at %s: %w", xrayPath, err)
	}

	if err := seedApplyRequestOnServiceStart(apply.RoleServer, liveConfigDir, desiredExtensionsDir); err != nil {
		return err
	}
	rollback, pendingApplied, request, err := applyPendingIfRequested(apply.RoleServer)
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
		if errors.Is(retErr, winnet.ErrTunIPv4TentativeTimeout) {
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
				Role:      apply.RoleServer,
				Reason:    retErr.Error(),
			}, config.AuditLogPath())
		}
	}()
	meta, metaErr := loadLiveRuntimeMeta(liveConfigDir)
	if metaErr != nil {
		return metaErr
	}
	desired := meta.Desired
	opts.TunEnabled = meta.TunEnabled
	opts.TunName = meta.TunName
	opts.TunMTU = meta.TunMTU
	opts.TunAddr = meta.TunAddr

	wintunPath := filepath.Join(installDir, layout.BinDirName, "wintun.dll")
	if err := preflight.CheckTun(ctx, preflight.TunConfig{
		Enabled:       opts.TunEnabled,
		Name:          opts.TunName,
		Addr:          opts.TunAddr,
		MTU:           opts.TunMTU,
		WintunDLLPath: wintunPath,
	}); err != nil {
		return err
	}

	if stat, err := os.Stat(liveConfigDir); err != nil || !stat.IsDir() {
		if err != nil {
			return fmt.Errorf("configuration directory not found at %s: %w", liveConfigDir, err)
		}
		return fmt.Errorf("%s is not a directory", liveConfigDir)
	}

	configDir := liveConfigDir
	if err := syncXrayAssets(ctx, meta, xrayPath, configDir); err != nil {
		return err
	}
	appliedState, err := loadServerAppliedState(appliedStatePath)
	if err != nil {
		return err
	}
	if !appliedState.matches(desired.Reverse, desired.Redirects, desired.Forwards, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr) {
		if err := saveServerAppliedState(appliedStatePath, desired.Reverse, desired.Redirects, desired.Forwards, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr); err != nil {
			return err
		}
	}

	// sendThrough is compiled into xray.json during apply.

	configureCmd := func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		}
	}

	onStart := func() error {
		if opts.TunEnabled {
			if windowsRoutesDisabled {
				waitCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()
				logging.Info("ensuring tun IPv4 (routes disabled)", "timeout", "120s")
				ifIndex, ip, err := winnet.EnsureTunIPv4(waitCtx, opts.TunName, opts.TunAddr, false)
				if err != nil {
					logging.Warn("tun IPv4 ensure failed", "err", err)
					if errors.Is(err, winnet.ErrTunIPv4TentativeTimeout) {
						return err
					}
					return nil
				}
				logging.Info("tun IPv4 ready", "ifIndex", ifIndex, "ip", ip)
				logging.Info("windows route apply disabled; skipping redirect routes")
				return nil
			}
			waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			ifIndex, ip, err := winnet.EnsureTunIPv4(waitCtx, opts.TunName, opts.TunAddr, false)
			if err != nil {
				logging.Warn("tun IPv4 wait failed; skipping route apply", "err", err)
				if errors.Is(err, winnet.ErrTunIPv4TentativeTimeout) {
					return err
				}
				return nil
			}
			if details, detailErr := winnet.InterfaceIPv4Details(ifIndex); detailErr == nil {
				logging.Info(
					"tun IPv4 ready; applying routes",
					"ifIndex", ifIndex,
					"ip", ip,
					"operStatus", winnet.InterfaceOperStatusName(details.OperStatus),
					"dadState", winnet.InterfaceDadStateName(details.DadState),
				)
			} else {
				logging.Info("tun IPv4 ready; applying routes", "ifIndex", ifIndex, "ip", ip)
			}
			if ctx.Err() != nil {
				return nil
			}
			go winnet.DisableIPv6BindingWithRetry(ctx, opts.TunName)
			if err := applyRedirectRoutes(opts.TunName, opts.TunAddr, desired.Redirects); err != nil {
				logging.Warn("redirect route setup failed", "err", err)
			}
		}
		return nil
	}

	onReady := func(readyCtx context.Context) error {
		addr, err := resolveServerSocksAddress(filepath.Join(configDir, layout.XrayConfigFileName))
		if err != nil {
			logging.Warn("server socks health check using defaults", "err", err)
		}
		if err := health.WaitForSocksProxy(readyCtx, addr, socksHealthTimeout, socksHealthInterval); err != nil {
			return err
		}
		return nil
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
	if runErr != nil && errors.Is(runErr, context.Canceled) {
		logging.Info("server run canceled")
		return nil
	}
	if runErr != nil && errors.Is(runErr, winnet.ErrTunIPv4TentativeTimeout) {
		logging.Warn("tun ready failed; mode change remains pending", "err", runErr)
		return runErr
	}
	return runErr
}
