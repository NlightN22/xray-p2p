//go:build !linux

package natredirect

import (
	"fmt"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func NewCommand(_ func() config.Config) *cobra.Command {
	cmd := &cobra.Command{Use: "nat-redirect", Short: "Manage transparent NAT redirect rules (Linux only)"}
	for _, action := range []string{"add", "remove", "list"} {
		action := action
		cmd.AddCommand(&cobra.Command{
			Use:   action,
			Short: fmt.Sprintf("%s transparent redirect rules", action),
			RunE: func(cmd *cobra.Command, _ []string) error {
				clioutput.SetErrorCodeContext(cmd.Context(), "unsupported_platform")
				return fmt.Errorf("nat-redirect is supported only on Linux and OpenWrt")
			},
		})
	}
	return cmd
}
