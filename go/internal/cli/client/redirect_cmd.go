package clientcmd

import "github.com/spf13/cobra"

func newClientRedirectCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redirect",
		Short: "Manage custom client redirects",
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
