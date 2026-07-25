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
	for _, action := range []string{"add", "remove", "list"} {
		action := action
		cmd.AddCommand(&cobra.Command{
			Use:   action,
			Short: fmt.Sprintf("%s DNS forward entries", action),
			RunE: func(cmd *cobra.Command, _ []string) error {
				clioutput.SetErrorCodeContext(cmd.Context(), "unsupported_platform")
				return fmt.Errorf("dns-forward is supported only on Linux and OpenWrt")
			},
		})
	}
	return cmd
}
