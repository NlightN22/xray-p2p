package clientcmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientReverseCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reverse",
		Short: "Inspect client reverse tunnels",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientReverseList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	bindClientReverseFlags(cmd)
	cmd.AddCommand(newClientReverseListCmd(cfg))
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
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
}

func runClientReverseList(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client reverse list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")

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
	}
	records, err := clientReverseListFunc(opts)
	if err != nil {
		logging.Error("xp2p client reverse list failed", "err", err)
		return 1
	}
	if len(records) == 0 {
		fmt.Println("No reverse tunnels configured.")
		return 0
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "TAG\tHOST\tUSER\tENDPOINT TAG\tROUTING-BRIDGE\tDIRECT RULE")
	for _, rec := range records {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
			rec.Tag,
			rec.Host,
			rec.User,
			rec.EndpointTag,
			reversePresenceLabel(rec.Bridge),
			reversePresenceLabel(rec.DirectRule),
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
