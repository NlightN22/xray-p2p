package servercmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func newServerForwardCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forward",
		Short: "Manage server dokodemo-door forwards",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitError{code: 1}
		},
	}
	cmd.AddCommand(
		newServerForwardAddCmd(cfg),
		newServerForwardRemoveCmd(cfg),
		newServerForwardListCmd(cfg),
	)
	return cmd
}

func newServerForwardAddCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a server dokodemo-door forward",
		RunE: func(cmd *cobra.Command, args []string) error {
			code := runServerForwardAdd(commandContext(cmd), cfg(), args)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "server installation directory")
	flags.String("config-dir", "", "server configuration directory name or absolute path")
	flags.String("target", "", "target IP:port to forward traffic to")
	flags.String("listen", "", "local listen address (default 127.0.0.1)")
	flags.Int("listen-port", 0, "local listen port (auto-select when omitted)")
	flags.String("proto", "", "protocol to forward (tcp, udp, both)")
	flags.Int("base-port", forward.DefaultBasePort, "first port to probe when auto-selecting")
	_ = cmd.MarkFlagRequired("target")
	return cmd
}

func newServerForwardRemoveCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a server forward",
		RunE: func(cmd *cobra.Command, args []string) error {
			code := runServerForwardRemove(commandContext(cmd), cfg(), args)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "server installation directory")
	flags.String("config-dir", "", "server configuration directory name or absolute path")
	flags.Int("listen-port", 0, "forward listen port")
	flags.String("tag", "", "forward tag")
	flags.String("remark", "", "forward remark")
	flags.Bool("ignore-missing", false, "do not fail when the forward rule does not exist")
	return cmd
}

func newServerForwardListCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List server forwards",
		RunE: func(cmd *cobra.Command, args []string) error {
			code := runServerForwardList(commandContext(cmd), cfg(), args)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "server installation directory")
	flags.String("config-dir", "", "server configuration directory name or absolute path")
	return cmd
}

func runServerForwardAdd(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server forward add", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name or absolute path")
	target := fs.String("target", "", "target IP:port to forward traffic to")
	listen := fs.String("listen", "", "local listen address")
	listenPort := fs.Int("listen-port", 0, "local listen port")
	proto := fs.String("proto", "", "protocol to forward (tcp, udp, both)")
	basePort := fs.Int("base-port", forward.DefaultBasePort, "first port to probe when auto-selecting")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server forward add: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server forward add: unexpected arguments", "args", fs.Args())
		return 2
	}
	if strings.TrimSpace(*target) == "" {
		logging.Error("xp2p server forward add: --target is required")
		return 2
	}
	parsedProto, err := forward.ParseProtocol(*proto)
	if err != nil {
		logging.Error("xp2p server forward add: invalid --proto", "err", err)
		return 2
	}

	result, err := server.AddForward(server.ForwardAddOptions{
		InstallDir:    firstNonEmpty(*path, cfg.Server.InstallDir),
		ConfigDir:     firstNonEmpty(*configDir, cfg.Server.ConfigDir),
		Target:        *target,
		ListenAddress: *listen,
		ListenPort:    *listenPort,
		Protocol:      parsedProto,
		BasePort:      *basePort,
	})
	if err != nil {
		logging.Error("xp2p server forward add failed", "err", err)
		return 1
	}
	logging.Info("xp2p server forward added",
		"listen", fmt.Sprintf("%s:%d", result.Rule.ListenAddress, result.Rule.ListenPort),
		"target", result.Rule.Target(),
		"protocol", result.Rule.NetworkValue(),
		"remark", result.Rule.Remark,
	)
	if !result.Routed {
		logging.Warn("xp2p server forward has no matching redirect; traffic will use the first tunnel")
	}
	return 0
}

func runServerForwardRemove(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server forward remove", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name or absolute path")
	listenPort := fs.Int("listen-port", 0, "forward listen port")
	tag := fs.String("tag", "", "forward tag filter")
	remark := fs.String("remark", "", "forward remark filter")
	ignoreMissing := fs.Bool("ignore-missing", false, "do not fail when missing")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server forward remove: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server forward remove: unexpected arguments", "args", fs.Args())
		return 2
	}

	selector := forward.Selector{
		ListenPort: *listenPort,
		Tag:        *tag,
		Remark:     *remark,
	}
	if selector.Empty() {
		logging.Error("xp2p server forward remove: --listen-port, --tag, or --remark is required")
		return 2
	}

	removed, err := server.RemoveForward(server.ForwardRemoveOptions{
		InstallDir: firstNonEmpty(*path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Server.ConfigDir),
		Selector:   selector,
	})
	if err != nil {
		if *ignoreMissing {
			logging.Warn("xp2p server forward remove skipped", "err", err)
			return 0
		}
		logging.Error("xp2p server forward remove failed", "err", err)
		return 1
	}
	logging.Info("xp2p server forward removed",
		"listen", fmt.Sprintf("%s:%d", removed.ListenAddress, removed.ListenPort),
		"target", removed.Target(),
		"protocol", removed.NetworkValue(),
		"remark", removed.Remark,
	)
	return 0
}

func runServerForwardList(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server forward list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name or absolute path")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server forward list: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server forward list: unexpected arguments", "args", fs.Args())
		return 2
	}

	rules, err := server.ListForwards(server.ForwardListOptions{
		InstallDir: firstNonEmpty(*path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Server.ConfigDir),
	})
	if err != nil {
		logging.Error("xp2p server forward list failed", "err", err)
		return 1
	}
	if len(rules) == 0 {
		fmt.Println("No forward rules configured.")
		return 0
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "LISTEN\tPROTOCOLS\tTARGET\tREMARK")
	for _, rule := range rules {
		fmt.Fprintf(writer, "%s:%d\t%s\t%s:%d\t%s\n",
			rule.ListenAddress, rule.ListenPort, rule.NetworkValue(), rule.TargetIP, rule.TargetPort, rule.Remark)
	}
	_ = writer.Flush()
	return 0
}
