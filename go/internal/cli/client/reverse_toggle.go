package clientcmd

import (
	"context"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/spf13/cobra"
)

func newClientReverseDisableCmd(cfg commandConfig) *cobra.Command {
	return newClientReverseToggleCmd(cfg, false)
}

func newClientReverseEnableCmd(cfg commandConfig) *cobra.Command {
	return newClientReverseToggleCmd(cfg, true)
}

func newClientReverseToggleCmd(cfg commandConfig, enabled bool) *cobra.Command {
	var all bool
	name := "disable"
	short := "Disable a client reverse tunnel"
	if enabled {
		name = "enable"
		short = "Enable a client reverse tunnel"
	}
	cmd := &cobra.Command{
		Use:   name + " [tag|user|host]",
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) != 1 {
				return cobra.ExactArgs(1)(cmd, args)
			}
			if all && len(args) != 0 {
				return cobra.MaximumNArgs(0)(cmd, args)
			}
			target := ""
			if len(args) > 0 {
				target = args[0]
			}
			code := runClientReverseToggle(commandContext(cmd), cfg(), target, all, enabled)
			return errorForCode(code)
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "enable or disable all reverse tunnels")
	return cmd
}

func runClientReverseToggle(_ context.Context, _ config.Config, target string, all bool, enabled bool) int {
	if err := clientReverseToggleFunc(client.ReverseSetEnabledOptions{Target: target, All: all, Enabled: enabled}); err != nil {
		logging.Error("xp2p client reverse toggle failed", "err", err)
		return 1
	}
	action := "disabled"
	if enabled {
		action = "enabled"
	}
	if all {
		logging.Info("xp2p client reverse tunnels " + action)
	} else {
		logging.Info("xp2p client reverse tunnel "+action, "target", strings.TrimSpace(target))
	}
	return 0
}
