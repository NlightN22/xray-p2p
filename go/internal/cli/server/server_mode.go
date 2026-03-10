package servercmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func newServerModeCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode [tun|proxy]",
		Short: "Switch server mode between TUN and proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			if configFlag := cmd.InheritedFlags().Lookup("config"); configFlag != nil && configFlag.Changed {
				forwarded = append(forwarded, "--config", configFlag.Value.String())
			}
			code := runServerMode(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "server installation directory")
	flags.StringP("config-dir", "D", "", "server configuration directory name")
	return cmd
}

func runServerMode(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server mode", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name")
	configPath := fs.String("config", "", "path to configuration file")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server mode: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() == 0 {
		mode, err := resolveServerMode(cfg)
		if err != nil {
			logging.Error("xp2p server mode: failed to resolve current mode", "err", err)
			return 1
		}
		logging.Info("xp2p server mode: current mode", "mode", mode)
		return 0
	}
	if fs.NArg() != 1 {
		logging.Error("xp2p server mode: specify tun or proxy")
		return 2
	}

	mode := strings.ToLower(strings.TrimSpace(fs.Arg(0)))
	tunEnabled, err := parseMode(mode)
	if err != nil {
		logging.Error("xp2p server mode: invalid mode", "err", err)
		return 2
	}

	installDir := firstNonEmpty(*path, cfg.Server.InstallDir)
	configDirName := firstNonEmpty(*configDir, cfg.Server.ConfigDir)
	if strings.TrimSpace(installDir) == "" {
		logging.Error("xp2p server mode: install directory is required")
		return 2
	}

	updatedPath, err := config.UpdateTunEnabled(*configPath, "server", tunEnabled)
	if err != nil {
		logging.Error("xp2p server mode: update config failed", "err", err)
		return 1
	}

	if err := serverModeFunc(server.ModeOptions{
		InstallDir: installDir,
		ConfigDir:  configDirName,
		TunEnabled: tunEnabled,
		TunName:    cfg.Server.TunName,
		TunMTU:     cfg.Server.TunMTU,
		TunAddr:    cfg.Server.TunAddr,
	}); err != nil {
		logging.Error("xp2p server mode: apply failed", "err", err)
		return 1
	}

	logging.Info("xp2p server mode updated", "mode", mode, "config", updatedPath)
	return 0
}

func resolveServerMode(cfg config.Config) (string, error) {
	path := config.ConfigPath(layout.ServerAppliedStateFileName)
	data, err := os.ReadFile(path)
	if err == nil {
		if mode := parseModeFromState(data); mode != "" {
			return mode, nil
		}
	}
	if cfg.Server.TunEnabled {
		return "tun", nil
	}
	return "proxy", nil
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
