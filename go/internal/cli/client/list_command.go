package clientcmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type clientListResult struct {
	Endpoints []clientEndpointResult `json:"endpoints"`
	Links     []string               `json:"links,omitempty"`
}

type clientEndpointResult struct {
	Hostname   string `json:"hostname"`
	Tag        string `json:"tag"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	TLSMode    string `json:"tls_mode"`
	ServerName string `json:"server_name"`
	Enabled    bool   `json:"enabled"`
}

func runClientList(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	pending := fs.Bool("pending", false, "list pending configuration")
	links := fs.Bool("link", false, "print client connection links")

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
		Pending:    *pending,
	}
	records, err := clientListFunc(opts)
	if err != nil {
		logging.Error("xp2p client list failed", "err", err)
		return 1
	}
	if clioutput.EnabledContext(ctx) {
		result := clientListResult{Endpoints: make([]clientEndpointResult, 0, len(records))}
		if *links {
			result.Links = make([]string, 0, len(records))
		}
		for _, record := range records {
			result.Endpoints = append(result.Endpoints, clientEndpointResult{
				Hostname:   record.Hostname,
				Tag:        record.Tag,
				Address:    record.Address,
				Port:       record.Port,
				User:       record.User,
				TLSMode:    record.TLSMode,
				ServerName: record.ServerName,
				Enabled:    !record.Disabled,
			})
			if *links && record.Link != "" {
				result.Links = append(result.Links, record.Link)
			}
		}
		if err := clioutput.SetResultContext(ctx, result); err != nil {
			logging.Error("xp2p client list: publish JSON result failed", "err", err)
			return 1
		}
		return 0
	}
	if len(records) == 0 {
		fmt.Println("No client endpoints configured.")
		return 0
	}

	if *links {
		for _, rec := range records {
			if rec.Link == "" {
				continue
			}
			fmt.Println(rec.Link)
		}
		return 0
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "HOSTNAME\tTAG\tADDRESS\tPORT\tUSER\tTLS MODE\tSERVER NAME\tSTATE")
	for _, rec := range records {
		state := "enabled"
		if rec.Disabled {
			state = "disabled"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			rec.Hostname, rec.Tag, rec.Address, rec.Port, rec.User, rec.TLSMode, rec.ServerName, state)
	}
	_ = writer.Flush()
	return 0
}
