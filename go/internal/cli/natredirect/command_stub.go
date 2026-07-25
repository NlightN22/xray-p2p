//go:build !linux

package natredirect

import (
	"fmt"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func NewCommand(_ func() config.Config) *cobra.Command {
	cmd := &cobra.Command{Use: "nat-redirect", Short: "Manage transparent NAT redirect rules (Linux only)"}
	add := newUnsupportedLeaf("add", "Add transparent redirect rules for a CIDR")
	add.Flags().StringP("cidr", "C", "", "destination CIDR")
	add.Flags().IntP("port", "P", 0, "dokodemo-door port to redirect to (auto-detected when omitted)")
	add.Flags().BoolP("print-only", "O", false, "render firewall changes without applying them")
	add.Flags().BoolP("quiet", "q", false, "avoid interactive prompts when auto-selecting dokodemo port")
	add.Flags().StringP("snippet", "s", defaultStubSnippet, "nftables snippet path")
	add.Flags().StringP("entry-dir", "E", defaultStubEntryDir, "entry directory for nftables snippet generation")
	add.Flags().StringP("inbounds", "i", defaultStubInbounds, "path to inbounds.json used for auto port detection")

	remove := newUnsupportedLeaf("remove", "Remove transparent redirect rules")
	remove.Flags().StringP("cidr", "C", "", "destination CIDR")
	remove.Flags().BoolP("all", "a", false, "remove all transparent redirects")
	remove.Flags().BoolP("print-only", "O", false, "render firewall changes without applying them")
	remove.Flags().StringP("snippet", "s", defaultStubSnippet, "nftables snippet path")
	remove.Flags().StringP("entry-dir", "E", defaultStubEntryDir, "entry directory for nftables snippet generation")

	list := newUnsupportedLeaf("list", "List transparent redirect entries")
	cmd.AddCommand(add, remove, list)
	return cmd
}

const (
	defaultStubSnippet  = "/etc/nftables.d/xray-transparent.nft"
	defaultStubEntryDir = "/etc/nftables.d/xray-transparent.d"
	defaultStubInbounds = layout.UnixConfigRoot + "/" + layout.ClientConfigDir + "/inbounds.json"
)

func newUnsupportedLeaf(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clioutput.SetErrorCodeContext(cmd.Context(), "unsupported_platform")
			return fmt.Errorf("nat-redirect is supported only on Linux and OpenWrt")
		},
	}
}
