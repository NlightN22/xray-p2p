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
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/health"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

// Run launches xray-core using the installed client configuration directory and blocks until the process exits.
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

	rollback, pendingApplied, err := applyPendingIfRequested(apply.RoleClient, liveConfigDir)
	if err != nil {
		return err
	}
	if pendingApplied {
		configPath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
		if cfg, err := config.Load(config.Options{Path: configPath}); err != nil {
			logging.Warn("xp2p: reload client config after apply failed", "err", err)
		} else {
			opts.TunEnabled = cfg.Client.TunEnabled
			opts.TunName = cfg.Client.TunName
			opts.TunMTU = cfg.Client.TunMTU
			opts.TunAddr = cfg.Client.TunAddr
			opts.TunMode = cfg.Client.TunMode
			opts.DNSServers = cfg.Client.DNSServers
			opts.FullTunnelVerbose = opts.FullTunnelVerbose || cfg.Client.FullTunnelVerbose
			opts.FullTunnelTag = cfg.Client.FullTunnelTag
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

	paths, err := resolveClientPaths(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	paths, err = adjustRunPaths(paths)
	if err != nil {
		return err
	}
	configDir := paths.configDir
	desired, err := loadClientInstallState(paths.configFile)
	if err != nil {
		return err
	}
	appliedState, err := loadClientAppliedState(paths.stateFile)
	if err != nil {
		return err
	}
	if !appliedState.matches(desired, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr) {
		if err := applyClientDesiredConfig(paths, desired, ModeOptions{
			InstallDir:    installDir,
			ConfigDir:     opts.ConfigDir,
			TunEnabled:    opts.TunEnabled,
			TunName:       opts.TunName,
			TunMTU:        opts.TunMTU,
			TunAddr:       opts.TunAddr,
			TunMode:       opts.TunMode,
			FullTunnelTag: opts.FullTunnelTag,
		}, true); err != nil {
			return err
		}
		if err := saveClientAppliedState(paths.stateFile, desired, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr); err != nil {
			return err
		}
	}

	wantFull := opts.TunEnabled && strings.EqualFold(strings.TrimSpace(opts.TunMode), "full")
	if !wantFull {
		if windowsRoutesDisabled {
			logging.Info("xp2p: windows route apply disabled; skipping full-tunnel restore")
		} else {
			if err := restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose); err != nil {
				return err
			}
		}
	}
	defer func() {
		if !wantFull {
			return
		}
		if windowsRoutesDisabled {
			logging.Info("xp2p: windows route apply disabled; skipping full-tunnel rollback")
		} else {
			if err := restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose); err != nil {
				logging.Warn("xp2p: full-tunnel rollback failed", "err", err)
			}
		}
	}()

	if err := updateSendThroughOutbound(ctx, paths, opts.TunEnabled); err != nil {
		return err
	}

	wintunPath := filepath.Join(installDir, layout.BinDirName, "wintun.dll")
	if result, err := winnet.CleanupWintunAdapter(wintunPath, opts.TunName); err != nil {
		logging.Warn("xp2p: wintun adapter cleanup failed", "interface", opts.TunName, "result", "error", "err", err)
	} else {
		logging.Info("xp2p: wintun adapter cleanup", "interface", opts.TunName, "result", result)
	}

	stopHeartbeat := startHeartbeatLoop(ctx, installDir, configDir, opts.Heartbeat)
	defer stopHeartbeat()

	configureCmd := func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
		}
	}

	onStart := func() error {
		if opts.TunEnabled {
			if windowsRoutesDisabled {
				waitCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()
				logging.Info("xp2p: ensuring tun IPv4 (routes disabled)", "timeout", "120s")
				ifIndex, ip, err := winnet.EnsureTunIPv4(waitCtx, opts.TunName, opts.TunAddr, opts.FullTunnelVerbose)
				if err != nil {
					logging.Warn("xp2p: tun IPv4 ensure failed", "err", err)
					if errors.Is(err, winnet.ErrTunIPv4TentativeTimeout) {
						return err
					}
					return nil
				}
				logging.Info("xp2p: tun IPv4 ready", "ifIndex", ifIndex, "ip", ip)
				if wantFull {
					logging.Info("xp2p: full-tunnel pending (routes disabled)")
				}
				logging.Info("xp2p: windows route apply disabled; skipping redirect/full-tunnel routes")
				return nil
			}
			waitCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			ifIndex, ip, err := winnet.EnsureTunIPv4(waitCtx, opts.TunName, opts.TunAddr, opts.FullTunnelVerbose)
			if err != nil {
				logging.Warn("xp2p: tun IPv4 ensure failed; skipping route apply", "err", err)
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
			if wantFull {
				if _, err := syncFullTunnel(ctx, paths, opts, desired); err != nil {
					logging.Warn("xp2p: full-tunnel apply failed", "err", err)
				}
			}
		}
		return nil
	}

	onReady := func(readyCtx context.Context) error {
		addr, err := resolveClientSocksAddress(paths.configFile)
		if err != nil {
			logging.Warn("xp2p: client socks health check using defaults", "err", err)
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
	if runErr != nil && pendingApplied && rollback != nil {
		if err := rollback.Restore(config.AuditLogPath()); err != nil {
			logging.Warn("xp2p: rollback failed after apply", "err", err)
		} else {
			logging.Warn("xp2p: rollback completed after apply failure")
		}
	}
	return runErr
}
