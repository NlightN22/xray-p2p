package clientcmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func runClientList(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client list: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client list: unexpected arguments", "args", fs.Args())
		return 2
	}

	opts := client.ListOptions{
		InstallDir: firstNonEmpty(*path, cfg.Client.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Client.ConfigDir),
	}
	records, err := clientListFunc(opts)
	if err != nil {
		logging.Error("xp2p client list failed", "err", err)
		return 1
	}
	if len(records) == 0 {
		fmt.Println("No client endpoints configured.")
		return 0
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "HOSTNAME\tTAG\tADDRESS\tPORT\tUSER\tALLOW INSECURE\tSERVER NAME")
	for _, rec := range records {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%s\t%t\t%s\n",
			rec.Hostname, rec.Tag, rec.Address, rec.Port, rec.User, rec.AllowInsecure, rec.ServerName)
	}
	_ = writer.Flush()
	return 0
}
