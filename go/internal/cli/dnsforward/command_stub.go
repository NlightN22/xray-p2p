//go:build !linux

package dnsforwardcmd

import (
	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func NewCommand(_ func() config.Config) *cobra.Command {
	return nil
}
