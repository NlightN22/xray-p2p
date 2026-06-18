package clientcmd

import (
	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/commandmeta"
)

func newClientRedirectCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redirect",
		Short: "Manage custom client redirects",
		Annotations: map[string]string{
			commandmeta.DefaultBehavior: "list configured redirect rules",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientRedirectList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.BoolP("pending", "y", false, "list pending configuration")
	cmd.AddCommand(
		newClientRedirectAddCmd(cfg),
		newClientRedirectDisableCmd(cfg),
		newClientRedirectEnableCmd(cfg),
		newClientRedirectRemoveCmd(cfg),
		newClientRedirectListCmd(cfg),
	)
	return cmd
}
