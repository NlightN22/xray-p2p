//go:build !linux

package dnsforwardcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func NewClientCommand(_ func() config.Config) *cobra.Command {
	return newUnsupportedCommand()
}

func NewServerCommand(_ func() config.Config) *cobra.Command {
	return newUnsupportedCommand()
}

func newUnsupportedCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "dns-forward", Short: "Manage dnsmasq forward entries on OpenWrt"}
	add := newUnsupportedLeaf("add", "Create or update a DNS forward entry")
	add.Flags().StringP("domain", "d", "", "domain name to match")
	add.Flags().StringP("target", "t", "", "upstream DNS server (IP:port)")
	add.Flags().BoolP("with-forward", "W", false, "deprecated; dns-forward always ensures a target forward")
	add.Flags().BoolP("intercept", "I", false, "install DNS intercept redirect (53/tcp,udp)")
	add.Flags().BoolP("quiet", "q", false, "suppress interactive prompts")
	add.Flags().BoolP("debug", "g", false, "emit diagnostics output on error")
	_ = add.MarkFlagRequired("domain")
	_ = add.MarkFlagRequired("target")

	remove := newUnsupportedLeaf("remove", "Remove a DNS forward entry")
	remove.Flags().StringP("domain", "d", "", "domain name to remove")
	remove.Flags().BoolP("with-forward", "W", false, "deprecated; auto-created target forwards are removed when unused")
	remove.Flags().BoolP("intercept", "I", false, "remove DNS intercept redirect")
	remove.Flags().BoolP("all", "a", false, "remove all managed DNS forward entries")
	remove.Flags().BoolP("quiet", "q", false, "suppress interactive prompts")
	remove.Flags().BoolP("debug", "g", false, "emit diagnostics output on error")

	list := newUnsupportedLeaf("list", "List managed DNS forwards")
	list.Flags().BoolP("debug", "g", false, "emit diagnostics output on error")
	cmd.AddCommand(add, remove, list)
	return cmd
}

func newUnsupportedLeaf(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clioutput.SetErrorCodeContext(cmd.Context(), "unsupported_platform")
			return fmt.Errorf("dns-forward is supported only on Linux and OpenWrt")
		},
	}
}
