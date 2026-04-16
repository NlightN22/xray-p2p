package clientcmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
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
		newClientServiceCmd(cfg),
		newClientStateCmd(cfg),
		newClientRenderCmd(cfg),
		newClientDebugCmd(cfg),
		newClientExportCmd(cfg),
		newClientImportCmd(cfg),
		newClientDeployCmd(cfg),
		newClientRedirectCmd(cfg),
		newClientForwardCmd(cfg),
		newClientReverseCmd(cfg),
		newClientModeCmd(cfg),
	)
	dnsForwardMaybeAdd(cmd, cfg)
	return cmd
}

func newClientInstallCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install xp2p client assets and reverse bridges",
		Long:  "Install xp2p client assets, register forward tunnels, and provision reverse bridges (<user><host>.rev) that reuse the server's sanitized user/host identifiers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientInstall(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.StringP("host", "H", "", "remote server host")
	flags.StringP("port", "P", "", "remote server port")
	flags.StringP("user", "u", "", "Trojan user email (used to derive the <user><host>.rev reverse bridge)")
	flags.StringP("password", "w", "", "Trojan password")
	flags.StringP("sni", "s", "", "TLS server name (SNI)")
	flags.StringP("link", "L", "", "Trojan client link (trojan://...)")
	flags.BoolP("allow-insecure", "I", false, "allow insecure TLS (skip verification)")
	flags.BoolP("strict-tls", "S", false, "enforce TLS verification")
	flags.BoolP("force", "f", false, "replace existing endpoint configuration")
	flags.StringP("tun-mode", "m", "", "TUN routing mode (split or full)")
	_ = cmd.RegisterFlagCompletionFunc("tun-mode", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"split", "full"}, cobra.ShellCompDirectiveNoFileComp
	})
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
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.BoolP("keep-files", "K", false, "keep installation files")
	flags.BoolP("ignore-missing", "m", false, "do not fail if installation is absent")
	flags.BoolP("all", "a", false, "remove all endpoints and configuration")
	flags.BoolP("quiet", "q", false, "do not prompt for removal")
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
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.BoolP("pending", "y", false, "list pending configuration")
	return cmd
}

func newClientRunCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run xp2p client in foreground",
		RunE: func(cmd *cobra.Command, args []string) error {
			if logLevel, ok, err := common.LogLevelFromFlags(cmd); err != nil {
				return err
			} else if ok {
				if err := common.ApplyProcessLogLevel(logLevel); err != nil {
					logging.Error("xp2p client run: invalid --log-level", "err", err)
					return errorForCode(2)
				}
			}
			forwarded := forwardFlags(cmd, args)
			code := runClientRun(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.BoolP("quiet", "q", false, "do not prompt for installation")
	flags.BoolP("auto-install", "A", false, "install automatically if missing")
	flags.BoolP("verbose", "V", false, "emit full-tunnel change details")
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
	flags.StringP("host", "H", "", "remote host (IP or DNS) to deploy")
	_ = cmd.MarkFlagRequired("host")
	flags.StringP("port", "P", "62025", "deploy port")
	flags.StringP("install-dir", "I", "", "server install directory override")
	flags.StringP("user", "u", "", "Trojan user identifier (email)")
	flags.StringP("password", "w", "", "Trojan user password (auto-generated when omitted)")
	flags.StringP("trojan-port", "T", "", "Trojan service port")
	flags.StringP("tun-mode", "m", "", "TUN routing mode (split or full)")
	flags.BoolP("force", "f", false, "allow changing existing tun mode")
	_ = cmd.RegisterFlagCompletionFunc("tun-mode", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"split", "full"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func forwardFlags(cmd *cobra.Command, args []string) []string {
	flags := cmd.Flags()
	localFlags := cmd.LocalFlags()
	forwarded := make([]string, 0, len(args)+flags.NFlag())
	flags.Visit(func(f *pflag.Flag) {
		if localFlags.Lookup(f.Name) == nil {
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
