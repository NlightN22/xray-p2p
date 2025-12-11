//go:build linux

package servercmd

import (
	dnsforwardcmd "github.com/NlightN22/xray-p2p/go/internal/cli/dnsforward"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/spf13/cobra"
)

func dnsForwardMaybeAdd(cmd *cobra.Command, cfg commandConfig) {
	if c := dnsforwardcmd.NewServerCommand(func() config.Config { return cfg() }); c != nil {
		cmd.AddCommand(c)
	}
}
