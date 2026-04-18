package servercmd

import (
	"time"

	"github.com/spf13/cobra"
)

func newServerDeployCmd(cfg commandConfig) *cobra.Command {
	opts := serverDeployOptions{
		Once: true,
	}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Listen for xp2p client deploy requests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := serverDeployFunc(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Listen, "listen", "n", ":62025", "deploy listen address")
	flags.StringVarP(&opts.Link, "link", "L", "", "deploy link (trojan://...)")
	flags.StringVarP(&opts.DiagPort, "diag-service-port", "P", "", "diagnostics service port")
	_ = cmd.MarkFlagRequired("link")
	flags.DurationVarP(&opts.Timeout, "timeout", "t", 10*time.Minute, "idle shutdown timeout")
	return cmd
}
