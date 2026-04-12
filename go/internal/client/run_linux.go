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
	"github.com/NlightN22/xray-p2p/go/internal/xray"
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

	appliedStatePath := filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName))
	hasAppliedState := false
	if info, err := os.Stat(appliedStatePath); err == nil {
		if info.IsDir() {
			return fmt.Errorf("xp2p: %s is a directory, expected client applied state file", appliedStatePath)
		}
		hasAppliedState = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: inspect client applied state %s: %w", appliedStatePath, err)
	}

	xrayPath, err := xray.ResolveBinaryPath()
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

	tunEnabled := opts.TunEnabled
	if tunEnabled {
		if err := openwrt.EnsureTunInterface(opts.TunName, opts.TunAddr); err != nil {
			return tunSetupErrorWithHint("client run", err)
		}
		if err := linuxnet.EnsureTunInterface(opts.TunName, opts.TunAddr, opts.TunMTU); err != nil {
			return tunSetupErrorWithHint("client run", err)
		}
	}

	if !appliedState.matches(desired, tunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr) {
		if err := applyClientDesiredConfig(paths, desired, ModeOptions{
			InstallDir:    installDir,
			ConfigDir:     opts.ConfigDir,
			TunEnabled:    tunEnabled,
			TunName:       opts.TunName,
			TunMTU:        opts.TunMTU,
			TunAddr:       opts.TunAddr,
			TunMode:       opts.TunMode,
			FullTunnelTag: opts.FullTunnelTag,
		}, true); err != nil {
			return err
		}
		if err := modemgr.ApplyNatRedirectMode(modeLabel(tunEnabled)); err != nil {
			return err
		}
		if err := saveClientAppliedState(paths.stateFile, desired, tunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr); err != nil {
			return err
		}
	}

	fullEnabled, err := syncFullTunnel(ctx, paths, opts, desired)
	if err != nil {
		return err
	}
	defer func() {
		if !fullEnabled {
			return
		}
		if err := restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose); err != nil {
			logging.Warn("xp2p: full-tunnel rollback failed", "err", err)
		}
	}()

	stopHeartbeat := startHeartbeatLoop(ctx, installDir, configDir, opts.Heartbeat)
	defer stopHeartbeat()

	runErr := runXrayWithConfig(
		ctx,
		xrayPath,
		configDir,
		configDir,
		nil,
		func() error {
			if !tunEnabled {
				return nil
			}
			go func() {
				if err := linuxnet.EnsureTunAddress(opts.TunName, opts.TunAddr, opts.TunMTU); err != nil {
					logging.Warn("xp2p: tun address setup failed", "interface", opts.TunName, "err", err)
				}
			}()
			go func() {
				if err := applyRedirectRoutes(opts.TunName, opts.TunAddr, desired.Redirects); err != nil {
					logging.Warn("xp2p: redirect route setup failed", "err", err)
				}
			}()
			return nil
		},
		func(readyCtx context.Context) error {
			addr, err := resolveClientSocksAddress(paths.configFile)
			if err != nil {
				logging.Warn("xp2p: client socks health check using defaults", "err", err)
			}
			return health.WaitForSocksProxy(readyCtx, addr, socksHealthTimeout, socksHealthInterval)
		},
	)
	if runErr != nil && pendingApplied && rollback != nil {
		if hasAppliedState {
			if err := rollback.Restore(config.AuditLogPath()); err != nil {
				logging.Warn("xp2p: rollback failed after apply", "err", err)
			} else {
				logging.Warn("xp2p: rollback completed after apply failure")
			}
		} else {
			logging.Warn("xp2p: rollback skipped; no applied state yet")
		}
	}
	return runErr
}

func tunSetupErrorWithHint(action string, err error) error {
	return fmt.Errorf("xp2p: tun setup failed during %s: %w (set XP2P_CLIENT_TUN_ENABLED=false or run \"xp2p client mode proxy\")", action, err)
}
