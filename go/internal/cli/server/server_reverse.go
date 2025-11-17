package servercmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func newServerReverseCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reverse",
		Short: "Inspect server reverse tunnels",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runServerReverseList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	bindServerReverseFlags(cmd)
	cmd.AddCommand(newServerReverseListCmd(cfg))
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
	flags.String("path", "", "server installation directory")
	flags.String("config-dir", "", "server configuration directory name or absolute path")
}

func runServerReverseList(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server reverse list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name or absolute path")

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
	}
	records, err := serverReverseListFunc(opts)
	if err != nil {
		logging.Error("xp2p server reverse list failed", "err", err)
		return 1
	}
	if len(records) == 0 {
		fmt.Println("No reverse tunnels configured.")
		return 0
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "DOMAIN\tHOST\tUSER\tOUTBOUND TAG\tPORTAL\tROUTING RULE")
	for _, rec := range records {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
			rec.Domain,
			rec.Host,
			rec.User,
			rec.Tag,
			serverReverseStatus(rec.Portal),
			serverReverseStatus(rec.RoutingRule),
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
