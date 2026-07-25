//go:build !linux

package clientcmd

import (
	dnsforwardcmd "github.com/NlightN22/xray-p2p/go/internal/cli/dnsforward"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/spf13/cobra"
)

func dnsForwardMaybeAdd(cmd *cobra.Command, cfg commandConfig) {
	cmd.AddCommand(dnsforwardcmd.NewClientCommand(func() config.Config { return cfg() }))
}
