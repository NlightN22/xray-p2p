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
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

// Run launches xray-core using the installed configuration directory and blocks until the process exits.
func Run(ctx context.Context, opts RunOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	liveConfigDir, err := ResolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	rollback, pendingApplied, err := applyPendingIfRequested(apply.RoleServer, liveConfigDir)
	if err != nil {
		return err
	}
	if pendingApplied {
		configPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
		if cfg, err := config.Load(config.Options{Path: configPath}); err != nil {
			logging.Warn("xp2p: reload server config after apply failed", "err", err)
		} else {
			opts.TunEnabled = cfg.Server.TunEnabled
			opts.TunName = cfg.Server.TunName
			opts.TunMTU = cfg.Server.TunMTU
			opts.TunAddr = cfg.Server.TunAddr
		}
	}

	xrayPath := filepath.Join(installDir, layout.BinDirName, "xray.exe")
	if _, err := os.Stat(xrayPath); err != nil {
		return fmt.Errorf("xp2p: xray binary not found at %s: %w", xrayPath, err)
	}

	if stat, err := os.Stat(liveConfigDir); err != nil || !stat.IsDir() {
		if err != nil {
			return fmt.Errorf("xp2p: configuration directory not found at %s: %w", liveConfigDir, err)
		}
		return fmt.Errorf("xp2p: %s is not a directory", liveConfigDir)
	}

	configDir, configFile, err := adjustRunPaths(liveConfigDir)
	if err != nil {
		return err
	}
	desired, err := loadServerDesiredConfigWithFallback(pendingConfigPath(), filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)))
	if err != nil {
		return err
	}
	appliedState, err := loadServerAppliedState(filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName)))
	if err != nil {
		return err
	}
	if !appliedState.matches(desired.Reverse, desired.Redirects, desired.Forwards, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr) {
		if err := applyServerDesiredConfig(installDir, configDir, desired, appliedState.Reverse, ModeOptions{
			InstallDir: installDir,
			ConfigDir:  opts.ConfigDir,
			TunEnabled: opts.TunEnabled,
			TunName:    opts.TunName,
			TunMTU:     opts.TunMTU,
			TunAddr:    opts.TunAddr,
		}, true); err != nil {
			return err
		}
		if err := saveServerAppliedState(filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName)), desired.Reverse, desired.Redirects, desired.Forwards, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr); err != nil {
			return err
		}
	}

	if err := updateSendThroughOutbound(ctx, configDir, opts.TunEnabled); err != nil {
		return err
	}

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
				logging.Info("xp2p: ensuring tun IPv4 (routes disabled)", "timeout", "120s")
				ifIndex, ip, err := winnet.EnsureTunIPv4(waitCtx, opts.TunName, opts.TunAddr, false)
				if err != nil {
					logging.Warn("xp2p: tun IPv4 ensure failed", "err", err)
					if errors.Is(err, winnet.ErrTunIPv4TentativeTimeout) {
						return err
					}
					return nil
				}
				logging.Info("xp2p: tun IPv4 ready", "ifIndex", ifIndex, "ip", ip)
				logging.Info("xp2p: windows route apply disabled; skipping redirect routes")
				return nil
			}
			waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			ifIndex, ip, err := winnet.WaitForTunIPv4(waitCtx, opts.TunName, opts.TunAddr, false)
			if err != nil {
				logging.Warn("xp2p: tun IPv4 wait failed; skipping route apply", "err", err)
				if errors.Is(err, winnet.ErrTunIPv4TentativeTimeout) {
					return err
				}
				return nil
			}
			if details, detailErr := winnet.InterfaceIPv4Details(ifIndex); detailErr == nil {
				logging.Info(
					"xp2p: tun IPv4 ready; applying routes",
					"ifIndex", ifIndex,
					"ip", ip,
					"operStatus", winnet.InterfaceOperStatusName(details.OperStatus),
					"dadState", winnet.InterfaceDadStateName(details.DadState),
				)
			} else {
				logging.Info("xp2p: tun IPv4 ready; applying routes", "ifIndex", ifIndex, "ip", ip)
			}
			if ctx.Err() != nil {
				return nil
			}
			go winnet.DisableIPv6BindingWithRetry(ctx, opts.TunName)
			if err := applyRedirectRoutes(opts.TunName, opts.TunAddr, desired.Redirects); err != nil {
				logging.Warn("xp2p: redirect route setup failed", "err", err)
			}
		}
		return nil
	}

	onReady := func(readyCtx context.Context) error {
		addr, err := resolveServerSocksAddress(configFile)
		if err != nil {
			logging.Warn("xp2p: server socks health check using defaults", "err", err)
		}
		return health.WaitForSocksProxy(readyCtx, addr, socksHealthTimeout, socksHealthInterval)
	}

	runErr := runXrayWithConfig(
		ctx,
		xrayPath,
		configDir,
		installDir,
		configureCmd,
		onStart,
		onReady,
	)
	if runErr != nil && errors.Is(runErr, context.Canceled) {
		logging.Info("xp2p: server run canceled")
		return nil
	}
	if runErr != nil && pendingApplied && rollback != nil {
		if errors.Is(runErr, winnet.ErrTunIPv4TentativeTimeout) {
			logging.Warn("xp2p: tun ready failed; mode change remains pending", "err", runErr)
			return runErr
		}
		if err := rollback.Restore(config.AuditLogPath()); err != nil {
			logging.Warn("xp2p: rollback failed after apply", "err", err)
		} else {
			logging.Warn("xp2p: rollback completed after apply failure")
		}
	}
	return runErr
}
