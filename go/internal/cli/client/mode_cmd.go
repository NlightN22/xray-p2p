package clientcmd

import (
	"strings"

	"github.com/spf13/cobra"
)

func newClientModeCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode [tun|proxy] [split|full]",
		Short: "Switch client mode between TUN and proxy (optional tun mode)",
		Args:  cobra.RangeArgs(0, 2),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return []string{"tun", "proxy"}, cobra.ShellCompDirectiveNoFileComp
			case 1:
				if strings.ToLower(strings.TrimSpace(args[0])) == "tun" {
					return []string{"split", "full"}, cobra.ShellCompDirectiveNoFileComp
				}
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
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
	flags.StringP("tag", "g", "", "outbound tag for full-tunnel routing (prompts when omitted)")
	flags.StringP("host", "H", "", "client endpoint hostname for full-tunnel routing")
	flags.BoolP("quiet", "q", false, "do not prompt for outbound tags")
	flags.BoolP("verbose", "V", false, "emit full-tunnel change details")
	_ = cmd.RegisterFlagCompletionFunc("tag", func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		tags := listClientModeCompletions(cfg(), cmd, true)
		return filterCompletions(tags, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("host", func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		hosts := listClientModeCompletions(cfg(), cmd, false)
		return filterCompletions(hosts, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func configPathFromInheritedFlag(cmd *cobra.Command) string {
	configFlag := cmd.InheritedFlags().Lookup("config")
	if configFlag == nil || !configFlag.Changed {
		return ""
	}
	return strings.TrimSpace(configFlag.Value.String())
}
