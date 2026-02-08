package clientcmd

import (
	"context"
	"errors"
	"flag"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/modemgr"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientModeCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode [tun|proxy]",
		Short: "Switch client mode between TUN and proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientMode(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	flags.String("config", "", "path to configuration file (toml)")
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
	if fs.NArg() != 1 {
		logging.Error("xp2p client mode: specify tun or proxy")
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

	if err := modemgr.ApplyNatRedirectMode(mode); err != nil {
		logging.Error("xp2p client mode: nat-redirect update failed", "err", err)
		return 1
	}

	logging.Info("xp2p client mode updated", "mode", mode, "config", updatedPath)
	return 0
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
