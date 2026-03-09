package root

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	clientcmd "github.com/NlightN22/xray-p2p/go/internal/cli/client"
	natredirectcmd "github.com/NlightN22/xray-p2p/go/internal/cli/natredirect"
	servercmd "github.com/NlightN22/xray-p2p/go/internal/cli/server"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
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
			_ = cmd.Help()
			return exitError{code: 1}
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
		return opts.ensureRuntime(cmd)
	}

	clientCmd := clientcmd.NewCommand(func() config.Config { return opts.cfg })

	serverCmd := servercmd.NewCommand(func() config.Config { return opts.cfg })

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
		newDiagCommand(func() config.Config { return opts.cfg }),
		newPingCommand(func() config.Config { return opts.cfg }),
		newCompletionCommand(rootCmd),
		newDocsCommand(rootCmd),
	)
	if natCmd := natredirectMaybeAdd(opts); natCmd != nil {
		rootCmd.AddCommand(natCmd)
	}

	return rootCmd
}

type rootOptions struct {
	configPath       string
	logLevel         string
	logJSON          bool
	versionRequested bool

	cfg       config.Config
	runtimeOK bool
}

func (o *rootOptions) bindGlobalFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.StringVarP(&o.configPath, "config", "c", "", "path to configuration file")
	flags.StringVarP(&o.logLevel, "log-level", "l", "", "override logging level")
	flags.BoolVarP(&o.logJSON, "log-json", "j", false, "emit logs in JSON format")
	flags.BoolVarP(&o.versionRequested, "version", "v", false, "print xp2p version and exit")
}

func (o *rootOptions) ensureRuntime(cmd *cobra.Command) error {
	if o.runtimeOK {
		return nil
	}
	cfg, err := config.Load(config.Options{
		Path:         strings.TrimSpace(o.configPath),
		Overrides:    o.buildOverrides(),
		AllowInvalid: shouldIgnoreInvalidConfig(cmd),
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

func shouldIgnoreInvalidConfig(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	switch cmd.CommandPath() {
	case "xp2p client install", "xp2p server install":
		return hasForceArg()
	default:
		return false
	}
}

func hasForceArg() bool {
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--force") {
			return true
		}
	}
	return false
}

func (o *rootOptions) buildOverrides() map[string]any {
	overrides := make(map[string]any)
	if lvl := strings.TrimSpace(o.logLevel); lvl != "" {
		overrides["logging.level"] = lvl
	}
	if o.logJSON {
		overrides["logging.format"] = "json"
	}
	return overrides
}

func natredirectMaybeAdd(opts *rootOptions) *cobra.Command {
	cmd := natredirectcmd.NewCommand(func() config.Config { return opts.cfg })
	return cmd
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
