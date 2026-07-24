package clientcmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientRedirectListCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured redirect rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientRedirectList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.BoolP("pending", "y", false, "list pending configuration")
	return cmd
}

func runClientRedirectList(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client redirect list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	pending := fs.Bool("pending", false, "list pending configuration")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client redirect list: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client redirect list: unexpected arguments", "args", fs.Args())
		return 2
	}

	opts := client.RedirectListOptions{
		InstallDir: firstNonEmpty(*path, cfg.Client.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Client.ConfigDir),
		Pending:    *pending,
	}
	records, err := clientRedirectListFunc(opts)
	if err != nil {
		logging.Error("xp2p client redirect list failed", "err", err)
		return 1
	}
	if clioutput.EnabledContext(ctx) {
		type redirectResult struct {
			Type        string `json:"type"`
			Value       string `json:"value"`
			OutboundTag string `json:"outbound_tag"`
			Host        string `json:"host"`
			Enabled     bool   `json:"enabled"`
		}
		result := struct {
			Redirects []redirectResult `json:"redirects"`
		}{Redirects: make([]redirectResult, 0, len(records))}
		for _, record := range records {
			result.Redirects = append(result.Redirects, redirectResult{
				Type: record.Type, Value: record.Value, OutboundTag: record.Tag,
				Host: record.Hostname, Enabled: !record.Disabled,
			})
		}
		if err := clioutput.SetResultContext(ctx, result); err != nil {
			logging.Error("xp2p client redirect list: publish JSON result failed", "err", err)
			return 1
		}
		return 0
	}
	if len(records) == 0 {
		fmt.Println("No redirect rules configured.")
		return 0
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "TYPE\tVALUE\tOUTBOUND TAG\tHOST\tSTATE")
	for _, rec := range records {
		state := "enabled"
		if rec.Disabled {
			state = "disabled"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", rec.Type, rec.Value, rec.Tag, rec.Hostname, state)
	}
	_ = writer.Flush()
	return 0
}
