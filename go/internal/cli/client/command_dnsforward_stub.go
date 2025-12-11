//go:build !linux

package clientcmd

import "github.com/spf13/cobra"

func dnsForwardMaybeAdd(cmd *cobra.Command, cfg commandConfig) {
	_ = cmd
	_ = cfg
}
