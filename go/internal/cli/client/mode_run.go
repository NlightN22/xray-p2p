package clientcmd

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/preflight"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func runClientMode(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client mode", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	configPath := fs.String("config", "", "path to configuration file")
	tag := fs.String("tag", "", "outbound tag for full-tunnel routing")
	host := fs.String("host", "", "client endpoint hostname for full-tunnel routing")
	quiet := fs.Bool("quiet", false, "do not prompt for outbound tags")
	verbose := fs.Bool("verbose", false, "emit full-tunnel change details")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client mode: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() == 0 {
		mode, err := resolveClientMode(cfg)
		if err != nil {
			logging.Error("xp2p client mode: failed to resolve current mode", "err", err)
			return 1
		}
		tunMode, err := resolveClientTunMode(*configPath, cfg)
		if err != nil {
			logging.Error("xp2p client mode: failed to resolve tun mode", "err", err)
			return 1
		}
		if mode == "tun" {
			status := ""
			if tunMode == "full" {
				enabled, err := client.FullTunnelEnabled(config.ConfigPath(layout.ClientFullTunnelStateFileName))
				if err != nil {
					logging.Warn("xp2p client mode: failed to read full-tunnel state", "err", err)
				} else if !enabled {
					status = "pending"
				}
			}
			if status != "" {
				logging.Info("xp2p client mode: current mode", "mode", mode, "tun_mode", tunMode, "tun_mode_status", status)
			} else {
				logging.Info("xp2p client mode: current mode", "mode", mode, "tun_mode", tunMode)
			}
			if err := clioutput.SetResultContext(ctx, struct {
				Mode          string `json:"mode"`
				TunMode       string `json:"tun_mode"`
				TunModeStatus string `json:"tun_mode_status"`
			}{Mode: mode, TunMode: tunMode, TunModeStatus: status}); err != nil {
				logging.Error("xp2p client mode: publish JSON result failed", "err", err)
				return 1
			}
		} else {
			logging.Info("xp2p client mode: current mode", "mode", mode)
			if err := clioutput.SetResultContext(ctx, struct {
				Mode          string `json:"mode"`
				TunMode       string `json:"tun_mode"`
				TunModeStatus string `json:"tun_mode_status"`
			}{Mode: mode}); err != nil {
				logging.Error("xp2p client mode: publish JSON result failed", "err", err)
				return 1
			}
		}
		return 0
	}
	if fs.NArg() > 2 {
		logging.Error("xp2p client mode: specify tun or proxy (optional split or full)")
		return 2
	}

	mode := strings.ToLower(strings.TrimSpace(fs.Arg(0)))
	tunEnabled, err := parseMode(mode)
	if err != nil {
		logging.Error("xp2p client mode: invalid mode", "err", err)
		return 2
	}

	installDir := firstNonEmpty(*path, cfg.Client.InstallDir)
	configDirName := firstNonEmpty(*configDir, cfg.Client.ConfigDir)
	if strings.TrimSpace(installDir) == "" {
		logging.Error("xp2p client mode: install directory is required")
		return 2
	}

	loadedCfg, err := loadModeConfig(*configPath, cfg)
	if err != nil {
		logging.Error("xp2p client mode: failed to load config", "err", err)
		return 1
	}

	if tunEnabled {
		wintunPath := filepath.Join(installDir, layout.BinDirName, "wintun.dll")
		if err := tunPreflightCheckFunc(ctx, preflight.TunConfig{
			Enabled:       true,
			Name:          loadedCfg.Client.TunName,
			Addr:          loadedCfg.Client.TunAddr,
			MTU:           loadedCfg.Client.TunMTU,
			Mode:          loadedCfg.Client.TunMode,
			WintunDLLPath: wintunPath,
		}); err != nil {
			logging.Error("xp2p client mode: tun preflight failed", "err", err)
			return 1
		}
	}

	tunMode := loadedCfg.Client.TunMode
	if fs.NArg() == 2 {
		if !tunEnabled {
			logging.Error("xp2p client mode: tun mode is only valid with tun")
			return 2
		}
		value := strings.ToLower(strings.TrimSpace(fs.Arg(1)))
		if value != "split" && value != "full" {
			logging.Error("xp2p client mode: invalid tun mode (use split or full)")
			return 2
		}
		tunMode = value
	}

	fullTunnelTag := strings.TrimSpace(loadedCfg.Client.FullTunnelTag)
	if tunEnabled && tunMode == "full" {
		resolvedTag, _, err := resolveFullTunnelBinding(installDir, configDirName, *tag, *host, fullTunnelTag, *quiet)
		if err != nil {
			logging.Error("xp2p client mode: failed to resolve full-tunnel endpoint", "err", err)
			return 1
		}
		if strings.TrimSpace(resolvedTag) != "" {
			fullTunnelTag = resolvedTag
		}
	}

	update := config.ClientModeUpdate{
		TunEnabled: tunEnabled,
	}
	if *verbose {
		update.SetFullTunnelVerbose = true
		update.FullTunnelVerbose = true
	}
	if fs.NArg() == 2 {
		update.SetTunMode = true
		update.TunMode = tunMode
	}
	if tunEnabled && tunMode == "full" && strings.TrimSpace(fullTunnelTag) != "" {
		update.SetFullTunnelTag = true
		update.FullTunnelTag = fullTunnelTag
	}

	updatedPath, err := config.UpdateClientMode(*configPath, update)
	if err != nil {
		logging.Error("xp2p client mode: update config failed", "err", err)
		return 1
	}

	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		logging.Error("xp2p client mode: apply request failed", "err", err)
		return 1
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		logging.Error("xp2p client mode: apply request failed", "err", err)
		return 1
	}
	if err := restartClientServiceIfActive(ctx); err != nil {
		logging.Error("xp2p client mode: restart failed", "err", err)
		return 1
	}

	if fs.NArg() == 2 {
		logging.Info("xp2p client mode updated", "mode", mode, "tun_mode", tunMode, "config", updatedPath)
	} else {
		logging.Info("xp2p client mode updated", "mode", mode, "config", updatedPath)
	}
	return 0
}

func isIgnorableServerResolveError(err error) bool {
	return errors.Is(err, server.ErrServerReverseMissing) ||
		errors.Is(err, server.ErrServerReverseNotFound) ||
		errors.Is(err, server.ErrServerReverseNotSpecified) ||
		errors.Is(err, server.ErrServerReverseAmbiguous)
}
