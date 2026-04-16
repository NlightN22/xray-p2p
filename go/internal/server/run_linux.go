//go:build linux

package server

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
	"github.com/NlightN22/xray-p2p/go/internal/xray"
)

// Run launches xray-core using the installed server configuration directory and blocks until completion.
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

	appliedStatePath := filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName))
	hasAppliedState := false
	if info, err := os.Stat(appliedStatePath); err == nil {
		if info.IsDir() {
			return fmt.Errorf("xp2p: %s is a directory, expected server applied state file", appliedStatePath)
		}
		hasAppliedState = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: inspect server applied state %s: %w", appliedStatePath, err)
	}

	xrayPath, err := xray.ResolveBinaryPath()
	if err != nil {
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
		logging.Warn("server run failed after apply", "err", retErr)
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

	if stat, err := os.Stat(liveConfigDir); err != nil || !stat.IsDir() {
		if err != nil {
			return fmt.Errorf("xp2p: configuration directory not found at %s: %w", liveConfigDir, err)
		}
		return fmt.Errorf("xp2p: %s is not a directory", liveConfigDir)
	}

	configDir := liveConfigDir
	configFile := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	appliedState, err := loadServerAppliedState(appliedStatePath)
	if err != nil {
		return err
	}

	tunEnabled := opts.TunEnabled
	if tunEnabled {
		if err := openwrt.EnsureTunInterface(opts.TunName, opts.TunAddr); err != nil {
			return tunSetupErrorWithHint("server run", err)
		}
		if err := linuxnet.EnsureTunInterface(opts.TunName, opts.TunAddr, opts.TunMTU); err != nil {
			return tunSetupErrorWithHint("server run", err)
		}
	}

	if !appliedState.matches(desired.Reverse, desired.Redirects, desired.Forwards, tunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr) {
		if err := modemgr.ApplyNatRedirectMode(modeLabel(tunEnabled)); err != nil {
			return err
		}
		if err := saveServerAppliedState(appliedStatePath, desired.Reverse, desired.Redirects, desired.Forwards, tunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr); err != nil {
			return err
		}
	}

	runErr := runXrayWithConfig(
		ctx,
		xrayPath,
		filepath.Join(configDir, layout.XrayConfigFileName),
		configDir,
		nil,
		func() error {
			if !tunEnabled {
				return nil
			}
			go func() {
				if err := linuxnet.EnsureTunAddress(opts.TunName, opts.TunAddr, opts.TunMTU); err != nil {
					logging.Warn("tun address setup failed", "interface", opts.TunName, "err", err)
				}
			}()
			go func() {
				if err := applyRedirectRoutes(opts.TunName, opts.TunAddr, desired.Redirects); err != nil {
					logging.Warn("redirect route setup failed", "err", err)
				}
			}()
			return nil
		},
		func(readyCtx context.Context) error {
			addr, err := resolveServerSocksAddress(filepath.Join(configDir, layout.XrayConfigFileName))
			if err != nil {
				logging.Warn("server socks health check using defaults", "err", err)
			}
			if err := health.WaitForSocksProxy(readyCtx, addr, socksHealthTimeout, socksHealthInterval); err != nil {
				return err
			}
			return nil
		},
	)
	if runErr != nil && errors.Is(runErr, context.Canceled) {
		logging.Info("server run canceled")
		return nil
	}
	return runErr
}

func tunSetupErrorWithHint(action string, err error) error {
	return fmt.Errorf("xp2p: tun setup failed during %s: %w (set XP2P_SERVER_TUN_ENABLED=false or run \"xp2p server mode proxy\")", action, err)
}
