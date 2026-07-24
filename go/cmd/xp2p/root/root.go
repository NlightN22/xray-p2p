package root

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	clientcmd "github.com/NlightN22/xray-p2p/go/internal/cli/client"
	natredirectcmd "github.com/NlightN22/xray-p2p/go/internal/cli/natredirect"
	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
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
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if !opts.jsonOutput {
			defaultHelp(cmd, args)
			return
		}
		var help bytes.Buffer
		originalOut := cmd.OutOrStdout()
		cmd.SetOut(&help)
		defaultHelp(cmd, args)
		cmd.SetOut(originalOut)
		_ = json.NewEncoder(originalOut).Encode(clioutput.Envelope{
			SchemaVersion: clioutput.SchemaVersion,
			Command:       cmd.CommandPath(),
			Result: struct {
				Help string `json:"help"`
			}{Help: help.String()},
		})
	})
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if opts.versionRequested {
			if cmd != rootCmd {
				err := fmt.Errorf("--version cannot be combined with subcommands")
				if opts.jsonOutput {
					_ = clioutput.WriteError(cmd.ErrOrStderr(), cmd.CommandPath(), "invalid_argument", err)
					return clioutput.MarkRendered(err)
				}
				return err
			}
			if opts.jsonOutput {
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(clioutput.Envelope{
					SchemaVersion: clioutput.SchemaVersion,
					Command:       rootCmd.CommandPath(),
					Result: struct {
						Version string `json:"version"`
					}{Version: version.Current()},
				}); err != nil {
					return err
				}
			} else {
				fmt.Println(version.Current())
			}
			return exitError{code: 0}
		}
		if shouldSkipRuntime(cmd) {
			return nil
		}
		if err := opts.ensureRuntime(cmd); err != nil {
			if opts.jsonOutput {
				_ = clioutput.WriteError(cmd.ErrOrStderr(), cmd.CommandPath(), "command_failed", err)
				return clioutput.MarkRendered(err)
			}
			return err
		}
		return nil
	}

	clientCmd := clientcmd.NewCommand(func() config.Config { return opts.cfg })

	serverCmd := servercmd.NewCommand(func() config.Config { return opts.cfg })

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		if err != nil {
			if opts.jsonOutput {
				_ = clioutput.WriteError(cmd.ErrOrStderr(), cmd.CommandPath(), "invalid_argument", err)
				return clioutput.MarkRendered(err)
			}
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
		newHeartbeatCommand(),
		newCompletionCommand(rootCmd),
		newDocsCommand(rootCmd),
	)
	if natCmd := natredirectMaybeAdd(opts); natCmd != nil {
		rootCmd.AddCommand(natCmd)
	}
	classifyOutputContracts(rootCmd)
	decorateOutputContracts(rootCmd, opts)

	return rootCmd
}

type rootOptions struct {
	configPath       string
	logLevel         string
	logJSON          bool
	jsonOutput       bool
	versionRequested bool

	cfg       config.Config
	runtimeOK bool
}

func (o *rootOptions) bindGlobalFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.StringVarP(&o.configPath, "config", "c", "", "path to configuration file")
	flags.StringVarP(&o.logLevel, "log-level", "l", "", "override logging level")
	flags.BoolVarP(&o.logJSON, "log-json", "j", false, "emit logs in JSON format")
	flags.BoolVarP(&o.jsonOutput, "json", "J", false, "emit command result as JSON")
	flags.BoolVarP(&o.versionRequested, "version", "v", false, "print xp2p version and exit")
	_ = cmd.RegisterFlagCompletionFunc("log-level", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"debug", "info", "warn", "error"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func (o *rootOptions) ensureRuntime(cmd *cobra.Command) error {
	if o.runtimeOK {
		return nil
	}
	if lvl := strings.TrimSpace(o.logLevel); lvl != "" {
		normalized, err := logging.NormalizeLevel(lvl)
		if err != nil {
			return err
		}
		o.logLevel = normalized
		if err := os.Setenv(logging.EnvLogLevel, normalized); err != nil {
			return fmt.Errorf("set %s: %w", logging.EnvLogLevel, err)
		}
	}
	cfg, err := config.Load(config.Options{
		Path:         strings.TrimSpace(o.configPath),
		Overrides:    o.buildOverrides(),
		AllowInvalid: shouldIgnoreInvalidConfig(cmd),
	})
	if err != nil {
		return err
	}

	logOutput := io.Writer(os.Stderr)
	if o.jsonOutput {
		logOutput = io.Discard
	}
	logging.Configure(logging.Options{
		Output: logOutput,
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
	case "xp2p client service run", "xp2p server service run":
		return true
	default:
		return false
	}
}

func shouldSkipRuntime(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd.CommandPath() == "xp2p heartbeat contract" {
		return true
	}
	switch cmd.Name() {
	case "completion", "docs", "command-map", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
		return true
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

func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}
