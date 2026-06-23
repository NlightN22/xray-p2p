package servercmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/ha"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func newServerHACmd(_ commandConfig) *cobra.Command {
	cmd := &cobra.Command{Use: "ha", Short: "Manage server HA topology"}
	cmd.AddCommand(&cobra.Command{Use: "status", Short: "Show committed HA generation", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		generation, err := server.LoadHAGeneration(config.ConfigPath("server.toml"))
		if err != nil {
			return err
		}
		if generation.Number == 0 {
			fmt.Println("No HA generation is configured.")
			return nil
		}
		fmt.Printf("Generation: %d\nGroup: %s\nMembers: %d\nChannels: %d\n", generation.Number, generation.Group.Tag, len(generation.ConfirmedMembers()), len(generation.Channels))
		return nil
	}})
	peer := &cobra.Command{Use: "peer", Short: "Manage trusted HA peers"}
	peer.AddCommand(&cobra.Command{Use: "add <id> <secret>", Short: "Add or update an HA peer", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		return server.UpsertHAPeer(config.ConfigPath("server.toml"), ha.Peer{ID: args[0], Secret: args[1]})
	}})
	peer.AddCommand(&cobra.Command{Use: "remove <id>", Short: "Remove an HA peer", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		return server.RemoveHAPeer(config.ConfigPath("server.toml"), args[0])
	}})
	cmd.AddCommand(peer)
	return cmd
}
