package clientcmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientForwardCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forward",
		Short: "Manage client dokodemo-door forwards",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitError{code: 1}
		},
	}
	cmd.AddCommand(
		newClientForwardAddCmd(cfg),
		newClientForwardRemoveCmd(cfg),
		newClientForwardListCmd(cfg),
	)
	return cmd
}

func newClientForwardAddCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a client dokodemo-door forward",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientForwardAdd(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	flags.String("target", "", "target IP:port to forward traffic to")
	flags.String("listen", "", "local listen address (default 127.0.0.1)")
	flags.Int("listen-port", 0, "local listen port (auto-select when omitted)")
	flags.String("proto", "", "protocol to forward (tcp, udp, both)")
	flags.Int("base-port", forward.DefaultBasePort, "first port to probe when auto-selecting")
	_ = cmd.MarkFlagRequired("target")
	return cmd
}

func newClientForwardRemoveCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a client dokodemo-door forward",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientForwardRemove(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	flags.Int("listen-port", 0, "forward listen port")
	flags.String("tag", "", "forward tag")
	flags.String("remark", "", "forward remark")
	flags.Bool("ignore-missing", false, "do not fail when the forward rule does not exist")
	return cmd
}

func newClientForwardListCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List client forwards",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientForwardList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	return cmd
}

func runClientForwardAdd(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client forward add", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	target := fs.String("target", "", "target IP:port to forward traffic to")
	listen := fs.String("listen", "", "local listen address")
	listenPort := fs.Int("listen-port", 0, "local listen port")
	proto := fs.String("proto", "", "protocol to forward (tcp, udp, both)")
	basePort := fs.Int("base-port", forward.DefaultBasePort, "first port to probe when auto-selecting")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client forward add: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client forward add: unexpected arguments", "args", fs.Args())
		return 2
	}
	if strings.TrimSpace(*target) == "" {
		logging.Error("xp2p client forward add: --target is required")
		return 2
	}
	parsedProto, err := forward.ParseProtocol(*proto)
	if err != nil {
		logging.Error("xp2p client forward add: invalid --proto", "err", err)
		return 2
	}

	result, err := client.AddForward(client.ForwardAddOptions{
		InstallDir:    firstNonEmpty(*path, cfg.Client.InstallDir),
		ConfigDir:     firstNonEmpty(*configDir, cfg.Client.ConfigDir),
		Target:        *target,
		ListenAddress: *listen,
		ListenPort:    *listenPort,
		Protocol:      parsedProto,
		BasePort:      *basePort,
	})
	if err != nil {
		logging.Error("xp2p client forward add failed", "err", err)
		return 1
	}
	logging.Info("xp2p client forward added",
		"listen", fmt.Sprintf("%s:%d", result.Rule.ListenAddress, result.Rule.ListenPort),
		"target", result.Rule.Target(),
		"protocol", result.Rule.NetworkValue(),
		"remark", result.Rule.Remark,
	)
	if !result.Routed {
		logging.Warn("xp2p client forward has no matching redirect; traffic will use the default tunnel")
	}
	return 0
}

func runClientForwardRemove(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client forward remove", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	listenPort := fs.Int("listen-port", 0, "forward listen port")
	tag := fs.String("tag", "", "forward tag filter")
	remark := fs.String("remark", "", "forward remark filter")
	ignoreMissing := fs.Bool("ignore-missing", false, "do not fail when missing")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client forward remove: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client forward remove: unexpected arguments", "args", fs.Args())
		return 2
	}

	selector := forward.Selector{
		ListenPort: *listenPort,
		Tag:        *tag,
		Remark:     *remark,
	}
	if selector.Empty() {
		logging.Error("xp2p client forward remove: --listen-port, --tag, or --remark is required")
		return 2
	}

	removed, err := client.RemoveForward(client.ForwardRemoveOptions{
		InstallDir: firstNonEmpty(*path, cfg.Client.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Client.ConfigDir),
		Selector:   selector,
	})
	if err != nil {
		if *ignoreMissing {
			logging.Warn("xp2p client forward remove skipped", "err", err)
			return 0
		}
		logging.Error("xp2p client forward remove failed", "err", err)
		return 1
	}
	logging.Info("xp2p client forward removed",
		"listen", fmt.Sprintf("%s:%d", removed.ListenAddress, removed.ListenPort),
		"target", removed.Target(),
		"protocol", removed.NetworkValue(),
		"remark", removed.Remark,
	)
	return 0
}

func runClientForwardList(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client forward list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client forward list: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client forward list: unexpected arguments", "args", fs.Args())
		return 2
	}

	rules, err := client.ListForwards(client.ForwardListOptions{
		InstallDir: firstNonEmpty(*path, cfg.Client.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Client.ConfigDir),
	})
	if err != nil {
		logging.Error("xp2p client forward list failed", "err", err)
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
