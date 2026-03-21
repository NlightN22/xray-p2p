//go:build linux

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/cli/modemgr"
	"github.com/NlightN22/xray-p2p/go/internal/config"
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

	configDir, err := ResolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	if stat, err := os.Stat(configDir); err != nil || !stat.IsDir() {
		if err != nil {
			return fmt.Errorf("xp2p: configuration directory not found at %s: %w", configDir, err)
		}
		return fmt.Errorf("xp2p: %s is not a directory", configDir)
	}

	paths, err := resolveClientPaths(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	desired, err := loadClientInstallState(paths.configFile)
	if err != nil {
		return err
	}
	applied, err := loadClientAppliedState(paths.stateFile)
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

	if !applied.matches(desired, tunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr) {
		if err := applyClientDesiredConfig(paths, desired, ModeOptions{
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

	xrayPath, err := xray.ResolveBinaryPath()
	if err != nil {
		return err
	}
	return runXrayWithConfig(
		ctx,
		xrayPath,
		configDir,
		configDir,
		opts.ErrorLogPath,
		resolveClientLogPath,
		nil,
		func() {
			if !tunEnabled {
				return
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
		},
	)
}

func tunSetupErrorWithHint(action string, err error) error {
	return fmt.Errorf("xp2p: tun setup failed during %s: %w (set XP2P_CLIENT_TUN_ENABLED=false or run \"xp2p client mode proxy\")", action, err)
}

func resolveClientLogPath(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("xp2p: log path is empty")
	}
	trimmed := strings.TrimSpace(filepath.Clean(raw))
	if trimmed == "" || trimmed == "." {
		return "", errors.New("xp2p: log path is empty")
	}
	if filepath.IsAbs(trimmed) {
		return trimmed, nil
	}

	rel := filepath.ToSlash(trimmed)
	rel = strings.TrimPrefix(rel, "logs/")
	if rel == "" || rel == "." {
		rel = "xp2p-client.log"
	}

	return filepath.Join(config.LogRoot(), rel), nil
}
