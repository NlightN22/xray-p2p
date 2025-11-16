package root

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func newForwardCommand(cfg func() config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forward",
		Short: "Manage dokodemo-door forwards",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitError{code: 1}
		},
	}
	cmd.AddCommand(
		newForwardAddCmd(cfg),
		newForwardRemoveCmd(cfg),
		newForwardListCmd(cfg),
	)
	return cmd
}

type forwardRole string

const (
	roleClient forwardRole = "client"
	roleServer forwardRole = "server"
)

func parseForwardRole(value string) (forwardRole, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "client":
		return roleClient, nil
	case "server":
		return roleServer, nil
	default:
		return "", fmt.Errorf("xp2p: --role must be client or server")
	}
}

type forwardCommonFlags struct {
	role      string
	path      string
	configDir string
}

type forwardAddFlags struct {
	forwardCommonFlags
	target     string
	listen     string
	listenPort int
	proto      string
	basePort   int
}

type forwardRemoveFlags struct {
	forwardCommonFlags
	listenPort    int
	tag           string
	remark        string
	ignoreMissing bool
}

type forwardListFlags struct {
	forwardCommonFlags
}

func newForwardAddCmd(cfg func() config.Config) *cobra.Command {
	var flags forwardAddFlags
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a dokodemo-door forward",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runForwardAdd(commandContext(cmd), cfg(), flags)
			return errorForCode(code)
		},
	}
	bindForwardCommonFlags(cmd.Flags(), &flags.forwardCommonFlags)
	cmd.Flags().StringVar(&flags.target, "target", "", "target IP:port to forward traffic to")
	cmd.Flags().StringVar(&flags.listen, "listen", "", "local listen address (default 127.0.0.1)")
	cmd.Flags().IntVar(&flags.listenPort, "listen-port", 0, "local listen port (auto-select when omitted)")
	cmd.Flags().StringVar(&flags.proto, "proto", "", "protocol to forward (tcp, udp, both)")
	cmd.Flags().IntVar(&flags.basePort, "base-port", forward.DefaultBasePort, "first port to probe when selecting automatically")
	_ = cmd.MarkFlagRequired("target")
	return cmd
}

func newForwardRemoveCmd(cfg func() config.Config) *cobra.Command {
	var flags forwardRemoveFlags
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a dokodemo-door forward",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runForwardRemove(commandContext(cmd), cfg(), flags)
			return errorForCode(code)
		},
	}
	bindForwardCommonFlags(cmd.Flags(), &flags.forwardCommonFlags)
	cmd.Flags().IntVar(&flags.listenPort, "listen-port", 0, "listen port filter")
	cmd.Flags().StringVar(&flags.tag, "tag", "", "forward tag filter")
	cmd.Flags().StringVar(&flags.remark, "remark", "", "forward remark filter")
	cmd.Flags().BoolVar(&flags.ignoreMissing, "ignore-missing", false, "do not fail when the forward rule does not exist")
	return cmd
}

func newForwardListCmd(cfg func() config.Config) *cobra.Command {
	var flags forwardListFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured dokodemo-door forwards",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runForwardList(commandContext(cmd), cfg(), flags)
			return errorForCode(code)
		},
	}
	bindForwardCommonFlags(cmd.Flags(), &flags.forwardCommonFlags)
	return cmd
}

func bindForwardCommonFlags(f *pflag.FlagSet, flags *forwardCommonFlags) {
	f.StringVar(&flags.role, "role", "", "target role (client or server)")
	f.StringVar(&flags.path, "path", "", "installation directory override")
	f.StringVar(&flags.configDir, "config-dir", "", "configuration directory name or path")
}

func runForwardAdd(_ context.Context, cfg config.Config, flags forwardAddFlags) int {
	role, err := parseForwardRole(flags.role)
	if err != nil {
		logging.Error("xp2p forward add: invalid role", "err", err)
		return 2
	}
	proto, err := forward.ParseProtocol(flags.proto)
	if err != nil {
		logging.Error("xp2p forward add: invalid protocol", "err", err)
		return 2
	}

	switch role {
	case roleClient:
		result, err := client.AddForward(client.ForwardAddOptions{
			InstallDir:    firstNonEmpty(flags.path, cfg.Client.InstallDir),
			ConfigDir:     firstNonEmpty(flags.configDir, cfg.Client.ConfigDir),
			Target:        flags.target,
			ListenAddress: flags.listen,
			ListenPort:    flags.listenPort,
			Protocol:      proto,
			BasePort:      flags.basePort,
		})
		if err != nil {
			logging.Error("xp2p forward add failed", "role", "client", "err", err)
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
	default:
		result, err := server.AddForward(server.ForwardAddOptions{
			InstallDir:    firstNonEmpty(flags.path, cfg.Server.InstallDir),
			ConfigDir:     firstNonEmpty(flags.configDir, cfg.Server.ConfigDir),
			Target:        flags.target,
			ListenAddress: flags.listen,
			ListenPort:    flags.listenPort,
			Protocol:      proto,
			BasePort:      flags.basePort,
		})
		if err != nil {
			logging.Error("xp2p forward add failed", "role", "server", "err", err)
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
	}
	return 0
}

func runForwardRemove(_ context.Context, cfg config.Config, flags forwardRemoveFlags) int {
	role, err := parseForwardRole(flags.role)
	if err != nil {
		logging.Error("xp2p forward remove: invalid role", "err", err)
		return 2
	}
	selector := forward.Selector{
		ListenPort: flags.listenPort,
		Tag:        flags.tag,
		Remark:     flags.remark,
	}
	if selector.Empty() {
		logging.Error("xp2p forward remove: --listen-port, --tag, or --remark is required")
		return 2
	}

	var removed forward.Rule
	switch role {
	case roleClient:
		removed, err = client.RemoveForward(client.ForwardRemoveOptions{
			InstallDir: firstNonEmpty(flags.path, cfg.Client.InstallDir),
			ConfigDir:  firstNonEmpty(flags.configDir, cfg.Client.ConfigDir),
			Selector:   selector,
		})
	default:
		removed, err = server.RemoveForward(server.ForwardRemoveOptions{
			InstallDir: firstNonEmpty(flags.path, cfg.Server.InstallDir),
			ConfigDir:  firstNonEmpty(flags.configDir, cfg.Server.ConfigDir),
			Selector:   selector,
		})
	}
	if err != nil {
		if flags.ignoreMissing {
			logging.Warn("xp2p forward remove skipped", "err", err)
			return 0
		}
		logging.Error("xp2p forward remove failed", "err", err)
		return 1
	}
	logging.Info("xp2p forward removed",
		"listen", fmt.Sprintf("%s:%d", removed.ListenAddress, removed.ListenPort),
		"target", removed.Target(),
		"protocol", removed.NetworkValue(),
	)
	return 0
}

func runForwardList(_ context.Context, cfg config.Config, flags forwardListFlags) int {
	role, err := parseForwardRole(flags.role)
	if err != nil {
		logging.Error("xp2p forward list: invalid role", "err", err)
		return 2
	}

	var rules []forward.Rule
	switch role {
	case roleClient:
		rules, err = client.ListForwards(client.ForwardListOptions{
			InstallDir: firstNonEmpty(flags.path, cfg.Client.InstallDir),
			ConfigDir:  firstNonEmpty(flags.configDir, cfg.Client.ConfigDir),
		})
	default:
		rules, err = server.ListForwards(server.ForwardListOptions{
			InstallDir: firstNonEmpty(flags.path, cfg.Server.InstallDir),
			ConfigDir:  firstNonEmpty(flags.configDir, cfg.Server.ConfigDir),
		})
	}
	if err != nil {
		logging.Error("xp2p forward list failed", "err", err)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func errorForCode(code int) error {
	if code == 0 {
		return nil
	}
	return exitError{code: code}
}
