package clientcmd

import "github.com/spf13/cobra"

func newClientRedirectCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redirect",
		Short: "Manage custom client redirects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitError{code: 1}
		},
	}
	cmd.AddCommand(
		newClientRedirectAddCmd(cfg),
		newClientRedirectRemoveCmd(cfg),
		newClientRedirectListCmd(cfg),
	)
	return cmd
}
