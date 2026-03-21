package clientcmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientModeCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode [tun|proxy] [split|full]",
		Short: "Switch client mode between TUN and proxy (optional tun mode)",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			if configFlag := cmd.InheritedFlags().Lookup("config"); configFlag != nil && configFlag.Changed {
				forwarded = append(forwarded, "--config", configFlag.Value.String())
			}
			code := runClientMode(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	return cmd
}

func runClientMode(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client mode", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	configPath := fs.String("config", "", "path to configuration file")

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
			logging.Info("xp2p client mode: current mode", "mode", mode, "tun_mode", tunMode)
		} else {
			logging.Info("xp2p client mode: current mode", "mode", mode)
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

	updatedPath, err := config.UpdateTunEnabled(*configPath, "client", tunEnabled)
	if err != nil {
		logging.Error("xp2p client mode: update config failed", "err", err)
		return 1
	}
	tunMode := ""
	if fs.NArg() == 2 {
		if !tunEnabled {
			logging.Error("xp2p client mode: tun mode is only valid with tun")
			return 2
		}
		tunMode = strings.ToLower(strings.TrimSpace(fs.Arg(1)))
		if _, err := config.UpdateTunMode(*configPath, "client", tunMode); err != nil {
			logging.Error("xp2p client mode: update tun mode failed", "err", err)
			return 1
		}
	}

	if err := clientModeFunc(client.ModeOptions{
		InstallDir: installDir,
		ConfigDir:  configDirName,
		TunEnabled: tunEnabled,
		TunName:    cfg.Client.TunName,
		TunMTU:     cfg.Client.TunMTU,
		TunAddr:    cfg.Client.TunAddr,
	}); err != nil {
		logging.Error("xp2p client mode: apply failed", "err", err)
		return 1
	}

	if tunMode != "" {
		logging.Info("xp2p client mode updated", "mode", mode, "tun_mode", tunMode, "config", updatedPath)
	} else {
		logging.Info("xp2p client mode updated", "mode", mode, "config", updatedPath)
	}
	return 0
}

func resolveClientMode(cfg config.Config) (string, error) {
	path := config.ConfigPath(layout.ClientAppliedStateFileName)
	data, err := os.ReadFile(path)
	if err == nil {
		if mode := parseModeFromState(data); mode != "" {
			return mode, nil
		}
	}
	if cfg.Client.TunEnabled {
		return "tun", nil
	}
	return "proxy", nil
}

func resolveClientTunMode(configPath string, cfg config.Config) (string, error) {
	trimmed := strings.TrimSpace(configPath)
	if trimmed == "" {
		if cfg.Client.TunMode != "" {
			return cfg.Client.TunMode, nil
		}
		trimmed = config.ConfigPath(layout.ClientConfigFileName)
	}
	loaded, err := config.Load(config.Options{
		Path:         trimmed,
		AllowInvalid: true,
	})
	if err != nil {
		return "", err
	}
	return loaded.Client.TunMode, nil
}

func parseModeFromState(data []byte) string {
	var state struct {
		Mode       string `json:"mode"`
		TunEnabled bool   `json:"tun_enabled"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return ""
	}
	mode := strings.ToLower(strings.TrimSpace(state.Mode))
	if mode == "tun" || mode == "proxy" {
		return mode
	}
	if state.TunEnabled {
		return "tun"
	}
	return "proxy"
}

func parseMode(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tun":
		return true, nil
	case "proxy":
		return false, nil
	default:
		return false, errors.New("use tun or proxy")
	}
}
