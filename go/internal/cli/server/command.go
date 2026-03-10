package servercmd

import (
	"context"
	"fmt"
	"time"

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
		newServerExportCmd(cfg),
		newServerImportCmd(cfg),
		newServerUserCmd(cfg),
		newServerRedirectCmd(cfg),
		newServerForwardCmd(cfg),
		newServerReverseCmd(cfg),
		newServerCertCmd(cfg),
		newServerDeployCmd(cfg),
		newServerModeCmd(cfg),
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
	flags.BoolVarP(&opts.Force, "force", "f", false, "overwrite existing installation")
	return cmd
}

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

func newServerRunCmd(cfg commandConfig) *cobra.Command {
	var opts serverRunCommandOptions
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run xp2p server in foreground",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerRun(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name")
	flags.StringVarP(&opts.DiagPort, "diag-service-port", "P", "", "diagnostics service port")
	flags.StringVarP(&opts.DiagMode, "diag-service-mode", "M", "", "diagnostics service startup mode (auto|manual)")
	flags.BoolVarP(&opts.AutoInstall, "auto-install", "A", false, "install server assets when missing without prompting")
	flags.BoolVarP(&opts.Quiet, "quiet", "q", false, "suppress interactive prompts")
	flags.StringVarP(&opts.XrayLogFile, "xray-log-file", "X", "", "append xray stderr output to file")
	return cmd
}

func newServerUserCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage Trojan users on the server",
	}

	cmd.AddCommand(
		newServerUserAddCmd(cfg),
		newServerUserRemoveCmd(cfg),
		newServerUserListCmd(cfg),
	)
	return cmd
}

func newServerUserAddCmd(cfg commandConfig) *cobra.Command {
	var opts serverUserAddOptions
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a Trojan user and reverse portal",
		Long:  "Add a Trojan user, update inbounds.json, and create a sanitized <user><host>.rev reverse portal/routing entry so clients can mirror the bridge automatically (disable with --no-reverse).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerUserAdd(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.StringVarP(&opts.UserID, "id", "i", "", "Trojan client identifier (derives the <id><host>.rev reverse tag)")
	flags.StringVarP(&opts.Password, "password", "w", "", "Trojan client password or pre-shared key (auto-generated when omitted)")
	flags.StringVarP(&opts.Key, "key", "k", "", "alias for --password")
	flags.StringVarP(&opts.LinkHost, "host", "H", "", "public host name or IP for generated connection link")
	flags.BoolVarP(&opts.NoReverse, "no-reverse", "n", false, "skip creating reverse portal/routing entries")
	flags.BoolVarP(&opts.Force, "force", "f", false, "overwrite existing user entry")
	return cmd
}

func newServerUserRemoveCmd(cfg commandConfig) *cobra.Command {
	var opts serverUserRemoveOptions
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a Trojan user",
		Long:  "Remove a Trojan user and clean up the matching <user><host>.rev reverse portal.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerUserRemove(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.StringVarP(&opts.UserID, "id", "i", "", "Trojan client identifier")
	flags.StringVarP(&opts.Host, "host", "H", "", "public host name or IP (defaults to server host)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newServerUserListCmd(cfg commandConfig) *cobra.Command {
	var opts serverUserListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured Trojan users",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerUserList(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.StringVarP(&opts.Host, "host", "H", "", "public host name or IP for generated connection links")
	return cmd
}

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
	return cmd
}

func newServerCertStateCmd(cfg commandConfig) *cobra.Command {
	var opts serverCertStateOptions
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Show TLS certificate status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerCertState(cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	return cmd
}

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
