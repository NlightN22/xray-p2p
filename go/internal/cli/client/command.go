package clientcmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

type exitError struct {
	code int
}

func (e exitError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
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

// commandConfig returns the configuration snapshot shared with child commands.
type commandConfig func() config.Config

// NewCommand builds the xp2p client command with Cobra subcommands.
func NewCommand(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "client",
		Short:         "Manage xp2p client installation",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitError{code: 1}
		},
	}

	cmd.AddCommand(
		newClientInstallCmd(cfg),
		newClientRemoveCmd(cfg),
		newClientListCmd(cfg),
		newClientRunCmd(cfg),
		newClientDeployCmd(cfg),
		newClientRedirectCmd(cfg),
	)
	return cmd
}

func newClientInstallCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install xp2p client assets and reverse bridges",
		Long:  "Install xp2p client assets, register forward tunnels, and provision reverse bridges (<user>.rev) that reuse the server's sanitized user identifiers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientInstall(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	flags.String("server-address", "", "remote server address")
	flags.String("server-port", "", "remote server port")
	flags.String("user", "", "Trojan user email (used to derive the <user>.rev reverse bridge)")
	flags.String("password", "", "Trojan password")
	flags.String("server-name", "", "TLS server name")
	flags.String("link", "", "Trojan client link (trojan://...)")
	flags.Bool("allow-insecure", false, "allow insecure TLS (skip verification)")
	flags.Bool("strict-tls", false, "enforce TLS verification")
	flags.Bool("force", false, "replace existing endpoint configuration")
	return cmd
}

func newClientRemoveCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove [hostname|tag]",
		Short: "Remove xp2p client endpoints or entire installation",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientRemove(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	flags.Bool("keep-files", false, "keep installation files")
	flags.Bool("ignore-missing", false, "do not fail if installation is absent")
	flags.Bool("all", false, "remove all endpoints and configuration")
	return cmd
}

func newClientListCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured xp2p client endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	return cmd
}

func newClientRunCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run xp2p client in foreground",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientRun(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	flags.Bool("quiet", false, "do not prompt for installation")
	flags.Bool("auto-install", false, "install automatically if missing")
	flags.String("xray-log-file", "", "file to append xray-core stderr output")
	return cmd
}

func newClientDeployCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy xp2p client via remote helper",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientDeploy(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.String("remote-host", "", "remote host (IP or DNS) to deploy")
	_ = cmd.MarkFlagRequired("remote-host")
	flags.String("deploy-port", "62025", "deploy port (default 62025)")
	flags.String("user", "", "Trojan user identifier (email)")
	flags.String("password", "", "Trojan user password (auto-generated when omitted)")
	flags.String("trojan-port", "", "Trojan service port")
	return cmd
}

func forwardFlags(cmd *cobra.Command, args []string) []string {
	disallowed := map[string]struct{}{
		"diag-service-port": {},
		"diag-service-mode": {},
	}

	flags := cmd.Flags()
	forwarded := make([]string, 0, len(args)+flags.NFlag())
	flags.Visit(func(f *pflag.Flag) {
		if _, skip := disallowed[f.Name]; skip {
			return
		}

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

func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}
