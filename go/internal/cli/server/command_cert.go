package servercmd

import "github.com/spf13/cobra"

func newServerCertCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Manage TLS certificates",
	}

	cmd.AddCommand(
		newServerCertStateCmd(cfg),
		newServerCertSetCmd(cfg),
	)
	return cmd
}

func newServerCertSetCmd(cfg commandConfig) *cobra.Command {
	var opts serverCertSetOptions
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set or replace TLS certificates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerCertSet(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.StringVarP(&opts.CertStore, "cert-store", "S", "", "TLS certificate store reference (win-store)")
	flags.StringVarP(&opts.Cert, "cert", "E", "", "TLS certificate file to deploy")
	flags.StringVarP(&opts.Key, "key", "k", "", "TLS private key file to deploy")
	flags.StringVarP(&opts.Host, "host", "H", "", "public host name or IP for certificate generation")
	flags.BoolVarP(&opts.Force, "force", "f", false, "overwrite existing TLS configuration without prompting")
	flags.BoolVarP(&opts.Quiet, "quiet", "q", false, "suppress interactive prompts")
	return cmd
}

func newServerCertStateCmd(cfg commandConfig) *cobra.Command {
	var opts serverCertStateOptions
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Show TLS certificate status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerCertState(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.BoolVarP(&opts.Pending, "pending", "y", false, "show pending configuration")
	return cmd
}
