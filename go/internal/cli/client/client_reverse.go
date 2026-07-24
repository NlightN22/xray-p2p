package clientcmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/commandmeta"
	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientReverseCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reverse",
		Short: "Inspect client reverse tunnels",
		Annotations: map[string]string{
			commandmeta.DefaultBehavior: "list client reverse tunnels",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientReverseList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	bindClientReverseFlags(cmd)
	cmd.AddCommand(
		newClientReverseDisableCmd(cfg),
		newClientReverseEnableCmd(cfg),
		newClientReverseListCmd(cfg),
	)
	return cmd
}

func newClientReverseListCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List client reverse tunnels",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientReverseList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	bindClientReverseFlags(cmd)
	return cmd
}

func bindClientReverseFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.BoolP("pending", "y", false, "list pending configuration")
}

func runClientReverseList(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client reverse list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	pending := fs.Bool("pending", false, "list pending configuration")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client reverse list: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client reverse list: unexpected arguments", "args", fs.Args())
		return 2
	}

	opts := client.ReverseListOptions{
		InstallDir: firstNonEmpty(*path, cfg.Client.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Client.ConfigDir),
		Pending:    *pending,
	}
	records, err := clientReverseListFunc(opts)
	if err != nil {
		logging.Error("xp2p client reverse list failed", "err", err)
		return 1
	}
	if clioutput.EnabledContext(ctx) {
		type reverseResult struct {
			Tag         string `json:"tag"`
			Host        string `json:"host"`
			User        string `json:"user"`
			EndpointTag string `json:"endpoint_tag"`
			Bridge      bool   `json:"routing_bridge_present"`
			DirectRule  bool   `json:"direct_rule_present"`
			Enabled     bool   `json:"enabled"`
		}
		result := struct {
			ReverseTunnels []reverseResult `json:"reverse_tunnels"`
		}{ReverseTunnels: make([]reverseResult, 0, len(records))}
		for _, record := range records {
			result.ReverseTunnels = append(result.ReverseTunnels, reverseResult{
				Tag: record.Tag, Host: record.Host, User: record.User, EndpointTag: record.EndpointTag,
				Bridge: record.Bridge, DirectRule: record.DirectRule, Enabled: !record.Disabled,
			})
		}
		if err := clioutput.SetResultContext(ctx, result); err != nil {
			logging.Error("xp2p client reverse list: publish JSON result failed", "err", err)
			return 1
		}
		return 0
	}
	if len(records) == 0 {
		fmt.Println("No reverse tunnels configured.")
		return 0
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "TAG\tHOST\tUSER\tENDPOINT TAG\tROUTING-BRIDGE\tDIRECT RULE\tSTATE")
	for _, rec := range records {
		state := "enabled"
		if rec.Disabled {
			state = "disabled"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			rec.Tag,
			rec.Host,
			rec.User,
			rec.EndpointTag,
			reversePresenceLabel(rec.Bridge),
			reversePresenceLabel(rec.DirectRule),
			state,
		)
	}
	_ = writer.Flush()
	return 0
}

func reversePresenceLabel(ok bool) string {
	if ok {
		return "present"
	}
	return "missing"
}
