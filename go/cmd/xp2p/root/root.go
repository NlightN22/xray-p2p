package root

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	clientcmd "github.com/NlightN22/xray-p2p/go/internal/cli/client"
	servercmd "github.com/NlightN22/xray-p2p/go/internal/cli/server"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

// NewCommand constructs the xp2p root command backed by Cobra.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}
	rootCmd := &cobra.Command{
		Use:           "xp2p",
		Short:         "Cross-platform helper for XRAY-P2P",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.ensureRuntime(cmd.Context()); err != nil {
				return err
			}
			return opts.runService(cmd.Context())
		},
	}

	opts.bindFlags(rootCmd)
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if opts.versionRequested {
			if cmd != rootCmd {
				return fmt.Errorf("--version cannot be combined with subcommands")
			}
			fmt.Println(version.Current())
			return exitError{code: 0}
		}
		return opts.ensureRuntime(cmd.Context())
	}

	rootCmd.AddCommand(
		clientcmd.NewCommand(func() config.Config { return opts.cfg }),
		servercmd.NewCommand(func() config.Config { return opts.cfg }),
		newPingCommand(func() config.Config { return opts.cfg }),
		newCompletionCommand(rootCmd),
		newDocsCommand(rootCmd),
	)

	return rootCmd
}

type rootOptions struct {
	configPath          string
	logLevel            string
	serverPort          string
	serverInstallDir    string
	serverConfigDir     string
	serverMode          string
	serverCert          string
	serverKey           string
	serverHost          string
	logJSON             bool
	clientInstallDir    string
	clientConfigDir     string
	clientServerAddr    string
	clientServerPort    string
	clientUser          string
	clientPassword      string
	clientServerName    string
	clientAllowInsecure bool
	clientStrictTLS     bool
	versionRequested    bool

	cfg       config.Config
	runtimeOK bool
}

func (o *rootOptions) bindFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.StringVar(&o.configPath, "config", "", "path to configuration file")
	flags.StringVar(&o.logLevel, "log-level", "", "override logging level")
	flags.StringVar(&o.serverPort, "server-port", "", "diagnostics service port")
	flags.StringVar(&o.serverInstallDir, "server-install-dir", "", "server installation directory (Windows)")
	flags.StringVar(&o.serverConfigDir, "server-config-dir", "", "server configuration directory name")
	flags.StringVar(&o.serverMode, "server-mode", "", "server startup mode (auto|manual)")
	flags.StringVar(&o.serverCert, "server-cert", "", "path to TLS certificate file (PEM)")
	flags.StringVar(&o.serverKey, "server-key", "", "path to TLS private key file (PEM)")
	flags.StringVar(&o.serverHost, "server-host", "", "public host name or IP for server certificate and links")
	flags.BoolVar(&o.logJSON, "log-json", false, "emit logs in JSON format")
	flags.StringVar(&o.clientInstallDir, "client-install-dir", "", "client installation directory (Windows)")
	flags.StringVar(&o.clientConfigDir, "client-config-dir", "", "client configuration directory name")
	flags.StringVar(&o.clientServerAddr, "client-server-address", "", "remote server address for client config")
	flags.StringVar(&o.clientServerPort, "client-server-port", "", "remote server port for client config")
	flags.StringVar(&o.clientUser, "client-user", "", "Trojan user email for client config")
	flags.StringVar(&o.clientPassword, "client-password", "", "Trojan password for client config")
	flags.StringVar(&o.clientServerName, "client-server-name", "", "TLS server name for client config")
	flags.BoolVar(&o.clientAllowInsecure, "client-allow-insecure", false, "allow TLS verification to be skipped for client config")
	flags.BoolVar(&o.clientStrictTLS, "client-strict-tls", false, "enforce TLS verification for client config")
	flags.BoolVar(&o.versionRequested, "version", false, "print xp2p version and exit")
}

func (o *rootOptions) ensureRuntime(ctx context.Context) error {
	if o.runtimeOK {
		return nil
	}
	cfg, err := config.Load(config.Options{
		Path:      strings.TrimSpace(o.configPath),
		Overrides: o.buildOverrides(),
	})
	if err != nil {
		return err
	}

	logging.Configure(logging.Options{
		Level:  cfg.Logging.Level,
		Format: logFormatFromConfig(cfg.Logging.Format),
	})
	logging.Info("xp2p starting", "version", version.Current())

	o.cfg = cfg
	o.runtimeOK = true
	return nil
}

func (o *rootOptions) buildOverrides() map[string]any {
	overrides := make(map[string]any)
	if lvl := strings.TrimSpace(o.logLevel); lvl != "" {
		overrides["logging.level"] = lvl
	}
	if o.logJSON {
		overrides["logging.format"] = "json"
	}
	if port := strings.TrimSpace(o.serverPort); port != "" {
		overrides["server.port"] = port
	}
	if dir := strings.TrimSpace(o.serverInstallDir); dir != "" {
		overrides["server.install_dir"] = dir
	}
	if cfgDir := strings.TrimSpace(o.serverConfigDir); cfgDir != "" {
		overrides["server.config_dir"] = cfgDir
	}
	if mode := strings.TrimSpace(o.serverMode); mode != "" {
		overrides["server.mode"] = mode
	}
	if cert := strings.TrimSpace(o.serverCert); cert != "" {
		overrides["server.certificate"] = cert
	}
	if key := strings.TrimSpace(o.serverKey); key != "" {
		overrides["server.key"] = key
	}
	if host := strings.TrimSpace(o.serverHost); host != "" {
		overrides["server.host"] = host
	}
	if dir := strings.TrimSpace(o.clientInstallDir); dir != "" {
		overrides["client.install_dir"] = dir
	}
	if cfgDir := strings.TrimSpace(o.clientConfigDir); cfgDir != "" {
		overrides["client.config_dir"] = cfgDir
	}
	if addr := strings.TrimSpace(o.clientServerAddr); addr != "" {
		overrides["client.server_address"] = addr
	}
	if port := strings.TrimSpace(o.clientServerPort); port != "" {
		overrides["client.server_port"] = port
	}
	if user := strings.TrimSpace(o.clientUser); user != "" {
		overrides["client.user"] = user
	}
	if pwd := strings.TrimSpace(o.clientPassword); pwd != "" {
		overrides["client.password"] = pwd
	}
	if name := strings.TrimSpace(o.clientServerName); name != "" {
		overrides["client.server_name"] = name
	}
	if o.clientAllowInsecure {
		overrides["client.allow_insecure"] = true
	}
	if o.clientStrictTLS {
		overrides["client.allow_insecure"] = false
	}
	return overrides
}

func (o *rootOptions) runService(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := server.StartBackground(ctx, server.Options{Port: o.cfg.Server.Port}); err != nil {
		logging.Error("failed to start diagnostics service", "err", err)
		return exitError{code: 1}
	}
	logging.Info("diagnostics service started", "port", o.cfg.Server.Port)
	<-ctx.Done()
	logging.Info("diagnostics service stopped")
	return nil
}

func logFormatFromConfig(value string) logging.Format {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "json":
		return logging.FormatJSON
	default:
		return logging.FormatText
	}
}

type exitError struct {
	code int
}

func (e exitError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

func (e exitError) ExitCode() int {
	return e.code
}

func newCompletionCommand(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}
	return cmd
}

func newDocsCommand(rootCmd *cobra.Command) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate CLI reference documentation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := strings.TrimSpace(dir)
			if path == "" {
				return fmt.Errorf("--dir is required")
			}
			if err := os.MkdirAll(path, 0o755); err != nil {
				return fmt.Errorf("create docs directory: %w", err)
			}
			return doc.GenMarkdownTree(rootCmd, path)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "destination directory for generated docs")
	_ = cmd.MarkFlagRequired("dir")
	return cmd
}

func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}
