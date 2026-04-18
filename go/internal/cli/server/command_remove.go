package servercmd

import "github.com/spf13/cobra"

func newServerRemoveCmd(cfg commandConfig) *cobra.Command {
	var opts serverRemoveCommandOptions
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove xp2p server installation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerRemove(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name")
	flags.BoolVarP(&opts.KeepFiles, "keep-files", "K", false, "keep installation files")
	flags.BoolVarP(&opts.IgnoreMissing, "ignore-missing", "m", false, "do not fail if service or files are absent")
	flags.BoolVarP(&opts.Quiet, "quiet", "q", false, "do not prompt for removal")
	return cmd
}
