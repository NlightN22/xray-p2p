package servercmd

import (
	"fmt"
	"strconv"
	"strings"

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
		peers, err := server.ListHAPeers(config.ConfigPath("server.toml"))
		if err != nil {
			return err
		}
		localID, err := server.LoadHALocalPeerID(config.ConfigPath("server.toml"))
		if err != nil {
			return err
		}
		fmt.Printf("Generation: %d\nGroup: %s\nMembers: %d\nChannels: %d\nPeers: %d\nLocal peer: %s\n", generation.Number, generation.Group.Tag, len(generation.ConfirmedMembers()), len(generation.Channels), len(peers), localID)
		return nil
	}})
	group := &cobra.Command{Use: "group", Short: "Manage the HA group"}
	group.AddCommand(&cobra.Command{Use: "create <id> <tag>", Short: "Create an HA group", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.InitializeHAGroup(config.ConfigPath("server.toml"), ha.Group{ID: args[0], Tag: args[1], Selector: ha.Selector{Mode: "automatic", FailureThreshold: 2}})
		return err
	}})
	group.AddCommand(&cobra.Command{Use: "remove", Short: "Remove an HA group after channel rebind or disable", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		_, err := server.RemoveHAGroup(config.ConfigPath("server.toml"))
		return err
	}})
	group.AddCommand(&cobra.Command{Use: "inspect", Short: "Inspect HA group topology", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		generation, err := server.LoadHAGeneration(config.ConfigPath("server.toml"))
		if err != nil {
			return err
		}
		fmt.Printf("ID: %s\nTag: %s\nMode: %s\n", generation.Group.ID, generation.Group.Tag, generation.Group.Selector.Mode)
		return nil
	}})
	group.AddCommand(&cobra.Command{Use: "update <automatic|manual|disabled>", Short: "Set HA group selector mode", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.SetHAGroupMode(config.ConfigPath("server.toml"), args[0])
		return err
	}})
	cmd.AddCommand(group)
	cmd.AddCommand(&cobra.Command{Use: "sync", Short: "Synchronize the next HA generation with peers", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := server.SyncHAGeneration(commandContext(cmd), config.ConfigPath("server.toml"))
		return err
	}})
	peer := &cobra.Command{Use: "peer", Short: "Manage trusted HA peers"}
	allowInsecure := false
	peer.AddCommand(&cobra.Command{Use: "self <id>", Short: "Set the local HA peer identity", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		return server.SaveHALocalPeerID(config.ConfigPath("server.toml"), args[0])
	}})
	peerAdd := &cobra.Command{Use: "add <id> <endpoint> <secret>", Short: "Add or update an HA peer", Args: cobra.ExactArgs(3), RunE: func(_ *cobra.Command, args []string) error {
		return server.UpsertHAPeer(config.ConfigPath("server.toml"), ha.Peer{ID: args[0], Endpoint: args[1], Secret: args[2], AllowInsecure: allowInsecure})
	}}
	peerAdd.Flags().BoolVarP(&allowInsecure, "allow-insecure", "k", false, "allow an untrusted peer certificate")
	peer.AddCommand(peerAdd)
	peer.AddCommand(&cobra.Command{Use: "remove <id>", Short: "Remove an HA peer", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		return server.RemoveHAPeer(config.ConfigPath("server.toml"), args[0])
	}})
	peer.AddCommand(&cobra.Command{Use: "list", Short: "List HA peers", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		peers, err := server.ListHAPeers(config.ConfigPath("server.toml"))
		if err != nil {
			return err
		}
		for _, item := range peers {
			fmt.Printf("%s\t%s\n", item.ID, item.Endpoint)
		}
		return nil
	}})
	cmd.AddCommand(peer)
	channel := &cobra.Command{Use: "channel", Short: "Manage stable HA reverse channels"}
	channel.AddCommand(&cobra.Command{Use: "create <id> <tag> <domain>", Short: "Create a group-bound HA channel", Args: cobra.ExactArgs(3), RunE: func(_ *cobra.Command, args []string) error {
		generation, err := server.LoadHAGeneration(config.ConfigPath("server.toml"))
		if err != nil {
			return err
		}
		_, err = server.AddHAChannel(config.ConfigPath("server.toml"), ha.Channel{ID: args[0], Tag: args[1], Domain: args[2], Binding: ha.ChannelBinding{GroupTag: generation.Group.Tag}})
		return err
	}})
	channel.AddCommand(&cobra.Command{Use: "disable <id>", Short: "Disable an HA channel", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.RebindHAChannel(config.ConfigPath("server.toml"), args[0], "", "", true)
		return err
	}})
	channel.AddCommand(&cobra.Command{Use: "inspect <id>", Short: "Inspect an HA channel", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		generation, err := server.LoadHAGeneration(config.ConfigPath("server.toml"))
		if err != nil {
			return err
		}
		for _, item := range generation.Channels {
			if strings.EqualFold(item.ID, args[0]) {
				fmt.Printf("ID: %s\nTag: %s\nDomain: %s\nGroup: %s\nEndpoint: %s\nDisabled: %t\n", item.ID, item.Tag, item.Domain, item.Binding.GroupTag, item.Binding.EndpointTag, item.Binding.Disabled)
				return nil
			}
		}
		return fmt.Errorf("HA channel %q is not registered", args[0])
	}})
	channel.AddCommand(&cobra.Command{Use: "rebind <id> <group-tag|endpoint-tag>", Short: "Rebind an HA channel", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		generation, err := server.LoadHAGeneration(config.ConfigPath("server.toml"))
		if err != nil {
			return err
		}
		groupTag, endpointTag := "", args[1]
		if strings.EqualFold(args[1], generation.Group.Tag) {
			groupTag, endpointTag = generation.Group.Tag, ""
		}
		_, err = server.RebindHAChannel(config.ConfigPath("server.toml"), args[0], groupTag, endpointTag, false)
		return err
	}})
	channel.AddCommand(&cobra.Command{Use: "rebind-endpoint <id> <endpoint-tag>", Short: "Bind an HA channel to a physical endpoint", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.RebindHAChannel(config.ConfigPath("server.toml"), args[0], "", args[1], false)
		return err
	}})
	channel.AddCommand(&cobra.Command{Use: "finalize <id>", Short: "Finalize a disabled HA channel", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.FinalizeHAChannel(config.ConfigPath("server.toml"), args[0])
		return err
	}})
	cmd.AddCommand(channel)
	member := &cobra.Command{Use: "member", Short: "Manage HA group members"}
	member.AddCommand(&cobra.Command{Use: "remove <id>", Short: "Tombstone an HA member", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.TombstoneHAMember(config.ConfigPath("server.toml"), args[0])
		return err
	}})
	member.AddCommand(&cobra.Command{Use: "add <id> <tag> <host> <port> <profile>", Short: "Add a confirmed HA member", Args: cobra.ExactArgs(5), RunE: func(_ *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[3])
		if err != nil {
			return err
		}
		_, err = server.AddHAMember(config.ConfigPath("server.toml"), ha.Member{ID: args[0], Tag: args[1], Host: args[2], Port: port, Profile: args[4], Confirmed: true})
		return err
	}})
	member.AddCommand(&cobra.Command{Use: "reprioritize <id> <priority>", Short: "Change HA member priority", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		priority, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}
		_, err = server.ReprioritizeHAMember(config.ConfigPath("server.toml"), args[0], priority)
		return err
	}})
	member.AddCommand(&cobra.Command{Use: "list", Short: "List HA group members", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		generation, err := server.LoadHAGeneration(config.ConfigPath("server.toml"))
		if err != nil {
			return err
		}
		for _, item := range generation.Group.Members {
			state := "confirmed"
			if item.Tombstone {
				state = "tombstoned"
			}
			fmt.Printf("%s\t%s\t%d\t%s\n", item.ID, item.Tag, item.Priority, state)
		}
		return nil
	}})
	channel.AddCommand(&cobra.Command{Use: "list", Short: "List HA channels", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		generation, err := server.LoadHAGeneration(config.ConfigPath("server.toml"))
		if err != nil {
			return err
		}
		for _, item := range generation.Channels {
			state := "enabled"
			if item.Binding.Disabled {
				state = "disabled"
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", item.ID, item.Domain, item.Binding.GroupTag, state)
		}
		return nil
	}})
	cmd.AddCommand(member)
	return cmd
}
