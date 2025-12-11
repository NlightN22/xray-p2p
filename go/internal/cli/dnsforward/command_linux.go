//go:build linux

package dnsforwardcmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/dnsforward"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func NewClientCommand(cfg func() config.Config) *cobra.Command {
	return newCommand(func() (*dnsforward.Manager, error) {
		c := cfg()
		return dnsforward.NewClientManager(c.Client.InstallDir, c.Client.ConfigDir)
	})
}

func NewServerCommand(cfg func() config.Config) *cobra.Command {
	return newCommand(func() (*dnsforward.Manager, error) {
		c := cfg()
		return dnsforward.NewServerManager(c.Server.InstallDir, c.Server.ConfigDir)
	})
}

func newCommand(makeMgr func() (*dnsforward.Manager, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns-forward",
		Short: "Manage dnsmasq forward entries on OpenWrt",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitError{code: 1}
		},
	}
	cmd.AddCommand(
		newAddCmd(makeMgr),
		newRemoveCmd(makeMgr),
		newListCmd(makeMgr),
	)
	return cmd
}

func newAddCmd(makeMgr func() (*dnsforward.Manager, error)) *cobra.Command {
	var domain string
	var target string
	var withForward bool
	var intercept bool
	var quiet bool

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create or update a DNS forward entry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := makeMgr()
			if err != nil {
				logging.Error("xp2p dns-forward add failed", "err", err)
				return exitError{code: 1}
			}
			entry, err := manager.Add(cmd.Context(), dnsforward.AddOptions{
				Domain:      domain,
				Target:      target,
				WithForward: withForward,
				Intercept:   intercept,
				Quiet:       quiet,
			})
			if err != nil {
				logging.Error("xp2p dns-forward add failed", "err", err)
				logDiagnostics(manager, intercept)
				return exitError{code: 1}
			}
			logging.Info("xp2p dns-forward added",
				"domain", entry.Domain,
				"server", entry.Server,
				"target", entry.Target,
				"labels", strings.Join(entry.Labels, ","),
			)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&domain, "domain", "", "domain name to match")
	flags.StringVar(&target, "target", "", "upstream DNS server (IP:port)")
	flags.BoolVar(&withForward, "with-forward", false, "create or reuse a port forward for the target")
	flags.BoolVar(&intercept, "intercept", false, "install DNS intercept redirect (53/tcp,udp)")
	flags.BoolVar(&quiet, "quiet", false, "suppress interactive prompts")
	_ = cmd.MarkFlagRequired("domain")
	_ = cmd.MarkFlagRequired("target")
	return cmd
}

func newRemoveCmd(makeMgr func() (*dnsforward.Manager, error)) *cobra.Command {
	var domain string
	var withForward bool
	var intercept bool
	var all bool
	var quiet bool

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a DNS forward entry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := makeMgr()
			if err != nil {
				logging.Error("xp2p dns-forward remove failed", "err", err)
				return exitError{code: 1}
			}
			removed, err := manager.Remove(dnsforward.RemoveOptions{
				Domain:      domain,
				All:         all,
				WithForward: withForward,
				Intercept:   intercept,
				Quiet:       quiet,
			})
			if err != nil {
				logging.Error("xp2p dns-forward remove failed", "err", err)
				logDiagnostics(manager, intercept)
				return exitError{code: 1}
			}
			logging.Info("xp2p dns-forward removed", "domains", strings.Join(removed, ","))
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&domain, "domain", "", "domain name to remove")
	flags.BoolVar(&withForward, "with-forward", false, "remove an auto-created port forward")
	flags.BoolVar(&intercept, "intercept", false, "remove DNS intercept redirect")
	flags.BoolVar(&all, "all", false, "remove all managed DNS forward entries")
	flags.BoolVar(&quiet, "quiet", false, "suppress interactive prompts")
	return cmd
}

func newListCmd(makeMgr func() (*dnsforward.Manager, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List managed DNS forwards",
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager, err := makeMgr()
			if err != nil {
				logging.Error("xp2p dns-forward list failed", "err", err)
				return exitError{code: 1}
			}
			entries, _, err := manager.List()
			if err != nil {
				logging.Error("xp2p dns-forward list failed", "err", err)
				logDiagnostics(manager, false)
				return exitError{code: 1}
			}
			if len(entries) == 0 {
				fmt.Println("No dns-forward entries configured.")
				return nil
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "DOMAIN\tSERVER\tLABELS")
			for _, entry := range entries {
				fmt.Fprintf(writer, "%s\t%s\t%s\n", entry.Domain, entry.Server, strings.Join(entry.Labels, ","))
			}
			_ = writer.Flush()
			return nil
		},
	}
	return cmd
}

func logDiagnostics(manager *dnsforward.Manager, includeFirewall bool) {
	diag := manager.Diagnostics(includeFirewall)
	for key, value := range diag {
		logging.Info(key, "output", value)
	}
}
