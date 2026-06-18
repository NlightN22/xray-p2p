package clientcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/spf13/cobra"
)

type clientEndpointToggleOptions struct {
	All bool
}

func newClientDisableCmd(cfg commandConfig) *cobra.Command {
	return newClientEndpointToggleCmd(cfg, false)
}

func newClientEnableCmd(cfg commandConfig) *cobra.Command {
	return newClientEndpointToggleCmd(cfg, true)
}

func newClientEndpointToggleCmd(cfg commandConfig, enabled bool) *cobra.Command {
	var opts clientEndpointToggleOptions
	name := "disable"
	short := "Disable a client endpoint"
	if enabled {
		name = "enable"
		short = "Enable a client endpoint"
	}
	cmd := &cobra.Command{
		Use:   name + " [hostname|tag]",
		Short: short,
		Args: func(_ *cobra.Command, args []string) error {
			if opts.All {
				if len(args) > 0 {
					return fmt.Errorf("--all does not accept positional arguments")
				}
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("expected exactly one endpoint")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) > 0 {
				target = args[0]
			}
			code := runClientEndpointToggle(commandContext(cmd), cfg(), target, opts, enabled)
			return errorForCode(code)
		},
	}
	cmd.Flags().BoolVarP(&opts.All, "all", "a", false, "enable or disable all endpoints")
	return cmd
}

func runClientEndpointToggle(ctx context.Context, _ config.Config, target string, opts clientEndpointToggleOptions, enabled bool) int {
	err := client.SetEndpointEnabled(ctx, client.EndpointSetEnabledOptions{
		Target:  target,
		All:     opts.All,
		Enabled: enabled,
	})
	if err != nil {
		action := "disable"
		if enabled {
			action = "enable"
		}
		logging.Error("xp2p client "+action+" failed", "err", err)
		return 1
	}
	action := "disabled"
	if enabled {
		action = "enabled"
	}
	if opts.All {
		logging.Info("xp2p client endpoints " + action)
	} else {
		logging.Info("xp2p client endpoint "+action, "target", strings.TrimSpace(target))
	}
	return 0
}
