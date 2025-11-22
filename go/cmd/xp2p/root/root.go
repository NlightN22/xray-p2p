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
			if err := opts.ensureRuntime(); err != nil {
				return err
			}
			return opts.runService(cmd.Context())
		},
	}

	opts.bindGlobalFlags(rootCmd)
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if opts.versionRequested {
			if cmd != rootCmd {
				return fmt.Errorf("--version cannot be combined with subcommands")
			}
			fmt.Println(version.Current())
			return exitError{code: 0}
		}
		return opts.ensureRuntime()
	}

	clientCmd := clientcmd.NewCommand(func() config.Config { return opts.cfg })
	opts.bindClientOverrideFlags(clientCmd)

	serverCmd := servercmd.NewCommand(func() config.Config { return opts.cfg })
	opts.bindServerOverrideFlags(serverCmd)

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		if err != nil {
			cmd.PrintErrln(err)
		}
		cmd.PrintErrln()
		_ = cmd.Usage()
		return err
	})

	rootCmd.AddCommand(
		clientCmd,
		serverCmd,
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

func (o *rootOptions) bindGlobalFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.StringVarP(&o.configPath, "config", "c", "", "path to configuration file")
	flags.StringVarP(&o.logLevel, "log-level", "l", "", "override logging level")
	flags.BoolVarP(&o.logJSON, "log-json", "j", false, "emit logs in JSON format")
	flags.BoolVarP(&o.versionRequested, "version", "v", false, "print xp2p version and exit")
	flags.StringVarP(&o.serverPort, "diag-service-port", "P", "", "diagnostics service port")
	flags.StringVarP(&o.serverMode, "diag-service-mode", "M", "", "diagnostics service startup mode (auto|manual)")
}

func (o *rootOptions) bindClientOverrideFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.StringVarP(&o.clientInstallDir, "client-install-dir", "I", "", "client installation directory (Windows)")
	flags.StringVarP(&o.clientConfigDir, "client-config-dir", "D", "", "client configuration directory name")
	flags.StringVarP(&o.clientServerAddr, "client-host", "A", "", "remote server host for client config")
	flags.StringVarP(&o.clientServerPort, "client-server-port", "R", "", "remote server port for client config")
	flags.StringVarP(&o.clientUser, "client-user", "U", "", "Trojan user email for client config")
	flags.StringVarP(&o.clientPassword, "client-password", "W", "", "Trojan password for client config")
	flags.StringVarP(&o.clientServerName, "client-server-name", "N", "", "TLS server name for client config")
	flags.BoolVarP(&o.clientAllowInsecure, "client-allow-insecure", "K", false, "allow TLS verification to be skipped for client config")
	flags.BoolVarP(&o.clientStrictTLS, "client-strict-tls", "T", false, "enforce TLS verification for client config")
}

func (o *rootOptions) bindServerOverrideFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.StringVarP(&o.serverInstallDir, "server-install-dir", "I", "", "server installation directory (Windows)")
	flags.StringVarP(&o.serverConfigDir, "server-config-dir", "D", "", "server configuration directory name")
	flags.StringVarP(&o.serverCert, "server-cert", "E", "", "path to TLS certificate file (PEM)")
	flags.StringVarP(&o.serverKey, "server-key", "K", "", "path to TLS private key file (PEM)")
	flags.StringVarP(&o.serverHost, "server-host", "H", "", "public host name or IP for server certificate and links")
}

func (o *rootOptions) ensureRuntime() error {
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
	if err := server.StartBackground(ctx, server.Options{
		Port:       o.cfg.Server.Port,
		InstallDir: o.cfg.Server.InstallDir,
	}); err != nil {
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
