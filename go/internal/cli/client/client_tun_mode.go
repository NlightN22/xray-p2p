package clientcmd

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientTunModeCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tun-mode [split|full]",
		Short: "Switch client TUN routing mode between split and full",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			if configFlag := cmd.InheritedFlags().Lookup("config"); configFlag != nil && configFlag.Changed {
				forwarded = append(forwarded, "--config", configFlag.Value.String())
			}
			code := runClientTunMode(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	return cmd
}

func runClientTunMode(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client tun-mode", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	configPath := fs.String("config", "", "path to configuration file")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client tun-mode: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() == 0 {
		mode, err := resolveClientTunMode(*configPath, cfg)
		if err != nil {
			logging.Error("xp2p client tun-mode: failed to resolve current mode", "err", err)
			return 1
		}
		logging.Info("xp2p client tun-mode: current mode", "mode", mode)
		return 0
	}
	if fs.NArg() != 1 {
		logging.Error("xp2p client tun-mode: specify split or full")
		return 2
	}

	mode := strings.ToLower(strings.TrimSpace(fs.Arg(0)))
	if mode != "split" && mode != "full" {
		logging.Error("xp2p client tun-mode: invalid mode (use split or full)")
		return 2
	}
	if mode == "full" && !cfg.Client.TunEnabled {
		logging.Warn("xp2p client tun-mode: full mode requires tun enabled")
	}

	updatedPath, err := config.UpdateTunMode(*configPath, "client", mode)
	if err != nil {
		logging.Error("xp2p client tun-mode: update config failed", "err", err)
		return 1
	}

	logging.Info("xp2p client tun-mode updated", "mode", mode, "config", updatedPath)
	return 0
}

func resolveClientTunMode(configPath string, cfg config.Config) (string, error) {
	path := strings.TrimSpace(configPath)
	if path == "" {
		return cfg.Client.TunMode, nil
	}
	loaded, err := config.Load(config.Options{Path: path})
	if err != nil {
		return "", err
	}
	return loaded.Client.TunMode, nil
}
