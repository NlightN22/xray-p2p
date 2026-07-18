package servercmd

import "github.com/spf13/cobra"

func newServerInstallCmd(cfg commandConfig) *cobra.Command {
	var opts serverInstallCommandOptions
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install xp2p server assets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerInstall(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name")
	flags.StringVarP(&opts.Port, "port", "P", "", "server listener port")
	flags.StringVarP(&opts.CertStore, "cert-store", "S", "", "TLS certificate store reference (win-store)")
	flags.StringVarP(&opts.Cert, "cert", "E", "", "TLS certificate file to deploy")
	flags.StringVarP(&opts.Key, "key", "k", "", "TLS private key file to deploy")
	flags.StringVarP(&opts.Host, "host", "H", "", "public host name or IP for generated configuration")
	flags.StringVarP(&opts.Profile, "profile", "r", "", "server tunnel profile")
	flags.BoolVarP(&opts.Force, "force", "f", false, "overwrite existing installation")
	return cmd
}
