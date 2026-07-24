package root

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
)

func newHeartbeatCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "heartbeat",
		Short: "Inspect the heartbeat protocol contract",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "contract",
		Short: "Print the machine-readable heartbeat contract",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(heartbeat.CurrentContract())
		},
	})
	return cmd
}
