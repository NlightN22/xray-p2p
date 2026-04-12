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
		if err := applyServerDesiredConfig(installDir, configDir, desired, appliedState.Reverse, ModeOptions{
			InstallDir: installDir,
			ConfigDir:  opts.ConfigDir,
			TunEnabled: tunEnabled,
			TunName:    opts.TunName,
			TunMTU:     opts.TunMTU,
			TunAddr:    opts.TunAddr,
		}, true); err != nil {
			return err
		}
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
			addr, err := resolveServerSocksAddress(configFile)
			if err != nil {
				logging.Warn("xp2p: server socks health check using defaults", "err", err)
			}
			return health.WaitForSocksProxy(readyCtx, addr, socksHealthTimeout, socksHealthInterval)
		},
	)
	if runErr != nil && errors.Is(runErr, context.Canceled) {
		logging.Info("xp2p: server run canceled")
		return nil
	}
	if runErr != nil && pendingApplied && rollback != nil && hasAppliedState {
		logging.Warn("xp2p: server run failed after apply", "err", runErr)
		if err := rollback.Restore(config.AuditLogPath()); err != nil {
			logging.Warn("xp2p: rollback failed after apply", "err", err)
		} else {
			logging.Warn("xp2p: rollback completed after apply failure")
		}
	} else if runErr != nil && pendingApplied && rollback != nil {
		logging.Warn("xp2p: rollback skipped; no applied state yet")
	}
	return runErr
}

func tunSetupErrorWithHint(action string, err error) error {
	return fmt.Errorf("xp2p: tun setup failed during %s: %w (set XP2P_SERVER_TUN_ENABLED=false or run \"xp2p server mode proxy\")", action, err)
}
