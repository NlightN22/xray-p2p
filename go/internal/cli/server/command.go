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
		newServerUserCmd(cfg),
		newServerRedirectCmd(cfg),
		newServerForwardCmd(cfg),
		newServerReverseCmd(cfg),
		newServerCertCmd(cfg),
		newServerDeployCmd(cfg),
	)

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
	flags.StringVar(&opts.Path, "path", "", "server installation directory")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "server configuration directory name")
	flags.StringVar(&opts.Port, "port", "", "server listener port")
	flags.StringVar(&opts.Cert, "cert", "", "TLS certificate file to deploy")
	flags.StringVar(&opts.Key, "key", "", "TLS private key file to deploy")
	flags.StringVar(&opts.Host, "host", "", "public host name or IP for generated configuration")
	flags.BoolVar(&opts.Force, "force", false, "overwrite existing installation")
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
	flags.StringVar(&opts.Path, "path", "", "server installation directory")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "server configuration directory name")
	flags.BoolVar(&opts.KeepFiles, "keep-files", false, "keep installation files")
	flags.BoolVar(&opts.IgnoreMissing, "ignore-missing", false, "do not fail if service or files are absent")
	flags.BoolVar(&opts.Quiet, "quiet", false, "do not prompt for removal")
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
	flags.StringVar(&opts.Path, "path", "", "server installation directory")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "server configuration directory name")
	flags.BoolVar(&opts.AutoInstall, "auto-install", false, "install server assets when missing without prompting")
	flags.BoolVar(&opts.Quiet, "quiet", false, "suppress interactive prompts")
	flags.StringVar(&opts.XrayLogFile, "xray-log-file", "", "append xray stderr output to file")
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
		Long:  "Add a Trojan user, update inbounds.json, and create a sanitized <user><host>.rev reverse portal/routing entry so clients can mirror the bridge automatically.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerUserAdd(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Path, "path", "", "server installation directory")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "server configuration directory name or absolute path")
	flags.StringVar(&opts.UserID, "id", "", "Trojan client identifier (derives the <id><host>.rev reverse tag)")
	flags.StringVar(&opts.Password, "password", "", "Trojan client password or pre-shared key")
	flags.StringVar(&opts.Key, "key", "", "alias for --password")
	flags.StringVar(&opts.LinkHost, "host", "", "public host name or IP for generated connection link")
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
	flags.StringVar(&opts.Path, "path", "", "server installation directory")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "server configuration directory name or absolute path")
	flags.StringVar(&opts.UserID, "id", "", "Trojan client identifier")
	flags.StringVar(&opts.Host, "host", "", "public host name or IP (defaults to server host)")
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
	flags.StringVar(&opts.Path, "path", "", "server installation directory")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "server configuration directory name or absolute path")
	flags.StringVar(&opts.Host, "host", "", "public host name or IP for generated connection links")
	return cmd
}

func newServerCertCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Manage TLS certificates",
	}

	cmd.AddCommand(newServerCertSetCmd(cfg))
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
	flags.StringVar(&opts.Path, "path", "", "server installation directory")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "server configuration directory name or absolute path")
	flags.StringVar(&opts.Cert, "cert", "", "TLS certificate file to deploy")
	flags.StringVar(&opts.Key, "key", "", "TLS private key file to deploy")
	flags.StringVar(&opts.Host, "host", "", "public host name or IP for certificate generation")
	flags.BoolVar(&opts.Force, "force", false, "overwrite existing TLS configuration without prompting")
	return cmd
}

func newServerDeployCmd(cfg commandConfig) *cobra.Command {
	var opts serverDeployOptions
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Listen for xp2p client deploy requests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := serverDeployFunc(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Listen, "listen", ":62025", "deploy listen address")
	flags.StringVar(&opts.Link, "link", "", "deploy link (xp2p+deploy://...)")
	_ = cmd.MarkFlagRequired("link")
	flags.BoolVar(&opts.Once, "once", true, "stop after a single deploy")
	flags.DurationVar(&opts.Timeout, "timeout", 10*time.Minute, "idle shutdown timeout")
	return cmd
}
