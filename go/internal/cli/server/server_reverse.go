package servercmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/commandmeta"
	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func newServerReverseCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reverse",
		Short: "Inspect server reverse tunnels",
		Annotations: map[string]string{
			commandmeta.DefaultBehavior: "list server reverse tunnels",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runServerReverseList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	bindServerReverseFlags(cmd)
	cmd.AddCommand(
		newServerReverseDisableCmd(cfg),
		newServerReverseEnableCmd(cfg),
		newServerReverseListCmd(cfg),
	)
	return cmd
}

func newServerReverseListCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List server reverse tunnels",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runServerReverseList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	bindServerReverseFlags(cmd)
	return cmd
}

func bindServerReverseFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "server installation directory")
	flags.StringP("config-dir", "D", "", "server configuration directory name or absolute path")
	flags.BoolP("pending", "y", false, "list pending configuration")
}

func runServerReverseList(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server reverse list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name or absolute path")
	pending := fs.Bool("pending", false, "list pending configuration")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server reverse list: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server reverse list: unexpected arguments", "args", fs.Args())
		return 2
	}

	opts := server.ReverseListOptions{
		InstallDir: firstNonEmpty(*path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Server.ConfigDir),
		Pending:    *pending,
	}
	records, err := serverReverseListFunc(opts)
	if err != nil {
		logging.Error("xp2p server reverse list failed", "err", err)
		return 1
	}
	if clioutput.EnabledContext(ctx) {
		type reverseResult struct {
			Domain      string `json:"domain"`
			Host        string `json:"host"`
			User        string `json:"user"`
			OutboundTag string `json:"outbound_tag"`
			Portal      bool   `json:"portal_present"`
			RoutingRule bool   `json:"routing_rule_present"`
			Enabled     bool   `json:"enabled"`
		}
		result := struct {
			ReverseTunnels []reverseResult `json:"reverse_tunnels"`
		}{ReverseTunnels: make([]reverseResult, 0, len(records))}
		for _, record := range records {
			result.ReverseTunnels = append(result.ReverseTunnels, reverseResult{
				Domain: record.Domain, Host: record.Host, User: record.User, OutboundTag: record.Tag,
				Portal: record.Portal, RoutingRule: record.RoutingRule, Enabled: !record.Disabled,
			})
		}
		if err := clioutput.SetResultContext(ctx, result); err != nil {
			logging.Error("xp2p server reverse list: publish JSON result failed", "err", err)
			return 1
		}
		return 0
	}
	if len(records) == 0 {
		fmt.Println("No reverse tunnels configured.")
		return 0
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "DOMAIN\tHOST\tUSER\tOUTBOUND TAG\tPORTAL\tROUTING RULE\tSTATE")
	for _, rec := range records {
		state := "enabled"
		if rec.Disabled {
			state = "disabled"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			rec.Domain,
			rec.Host,
			rec.User,
			rec.Tag,
			serverReverseStatus(rec.Portal),
			serverReverseStatus(rec.RoutingRule),
			state,
		)
	}
	_ = writer.Flush()
	return 0
}

func serverReverseStatus(ok bool) string {
	if ok {
		return "present"
	}
	return "missing"
}
