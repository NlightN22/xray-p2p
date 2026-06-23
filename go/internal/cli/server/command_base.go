package servercmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

type commandConfig func() config.Config

type exitError struct {
	code int
}

func (e exitError) Error() string {
	return "exit"
}

func (e exitError) ExitCode() int {
	return e.code
}

func errorForCode(code int) error {
	if code == 0 {
		return nil
	}
	return exitError{code: code}
}

func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

func NewCommand(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "server",
		Short:         "Manage xp2p server components",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newServerInstallCmd(cfg),
		newServerRemoveCmd(cfg),
		newServerRunCmd(cfg),
		newServerServiceCmd(cfg),
		newServerStateCmd(cfg),
		newServerRenderCmd(cfg),
		newServerDebugCmd(cfg),
		newServerExportCmd(cfg),
		newServerImportCmd(cfg),
		newServerUserCmd(cfg),
		newServerIdentityCmd(cfg),
		newServerRedirectCmd(cfg),
		newServerForwardCmd(cfg),
		newServerReverseCmd(cfg),
		newServerCertCmd(cfg),
		newServerDeployCmd(cfg),
		newServerModeCmd(cfg),
		newServerProfileCmd(cfg),
	)
	dnsForwardMaybeAdd(cmd, cfg)

	return cmd
}

func forwardFlags(cmd *cobra.Command, args []string) []string {
	flags := cmd.Flags()
	forwarded := make([]string, 0, len(args)+flags.NFlag())
	flags.Visit(func(f *pflag.Flag) {
		name := fmt.Sprintf("--%s", f.Name)
		if f.Value.Type() == "bool" {
			if f.Value.String() == "true" {
				forwarded = append(forwarded, name)
				return
			}
			forwarded = append(forwarded, fmt.Sprintf("%s=%s", name, f.Value.String()))
			return
		}
		forwarded = append(forwarded, fmt.Sprintf("%s=%s", name, f.Value.String()))
	})
	forwarded = append(forwarded, args...)
	return forwarded
}
