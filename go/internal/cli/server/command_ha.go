package servercmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/ha"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func haServerConfigPath() string {
	return config.ConfigPath(layout.ServerConfigFileName)
}

func newServerHACmd(_ commandConfig) *cobra.Command {
	cmd := &cobra.Command{Use: "ha", Short: "Manage server HA topology"}
	cmd.AddCommand(&cobra.Command{Use: "status", Short: "Show committed HA generation", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		generation, err := server.LoadHAGeneration(haServerConfigPath())
		if err != nil {
			return err
		}
		if generation.Number == 0 {
			if clioutput.Enabled(cmd) {
				return clioutput.SetResult(cmd, struct {
					Configured bool `json:"configured"`
				}{Configured: false})
			}
			fmt.Println("No HA generation is configured.")
			return nil
		}
		peers, err := server.ListHAPeers(haServerConfigPath())
		if err != nil {
			return err
		}
		localID, err := server.LoadHALocalPeerID(haServerConfigPath())
		if err != nil {
			return err
		}
		election := ha.ElectCoordinator(localID, peers)
		if clioutput.Enabled(cmd) {
			return clioutput.SetResult(cmd, struct {
				Configured       bool     `json:"configured"`
				Generation       uint64   `json:"generation"`
				Group            string   `json:"group"`
				MemberCount      int      `json:"member_count"`
				ChannelCount     int      `json:"channel_count"`
				PeerCount        int      `json:"peer_count"`
				LocalPeer        string   `json:"local_peer"`
				Coordinator      string   `json:"coordinator"`
				VotingMembership []string `json:"voting_membership"`
				Quorum           int      `json:"quorum"`
			}{true, generation.Number, generation.Group.Tag, len(generation.ConfirmedMembers()), len(generation.Channels), len(peers), localID, election.Coordinator, append([]string(nil), election.Voters...), election.Quorum})
		}
		fmt.Printf("Generation: %d\nGroup: %s\nMembers: %d\nChannels: %d\nPeers: %d\nLocal peer: %s\nCoordinator: %s\nVoting membership: %s\nQuorum: %d\n", generation.Number, generation.Group.Tag, len(generation.ConfirmedMembers()), len(generation.Channels), len(peers), localID, election.Coordinator, strings.Join(election.Voters, ","), election.Quorum)
		return nil
	}})
	group := &cobra.Command{Use: "group", Short: "Manage the HA group"}
	group.AddCommand(&cobra.Command{Use: "create <id> <tag>", Short: "Create an HA group", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.InitializeHAGroup(haServerConfigPath(), ha.Group{ID: args[0], Tag: args[1], Selector: ha.Selector{Mode: "automatic", FailureThreshold: 2}})
		return err
	}})
	group.AddCommand(&cobra.Command{Use: "remove", Short: "Remove an HA group after channel rebind or disable", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		_, err := server.RemoveHAGroup(haServerConfigPath())
		return err
	}})
	group.AddCommand(&cobra.Command{Use: "inspect", Short: "Inspect HA group topology", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		generation, err := server.LoadHAGeneration(haServerConfigPath())
		if err != nil {
			return err
		}
		if clioutput.Enabled(cmd) {
			return clioutput.SetResult(cmd, generation.Group)
		}
		fmt.Printf("ID: %s\nTag: %s\nMode: %s\n", generation.Group.ID, generation.Group.Tag, generation.Group.Selector.Mode)
		return nil
	}})
	group.AddCommand(&cobra.Command{Use: "update <automatic|manual|disabled>", Short: "Set HA group selector mode", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.SetHAGroupMode(haServerConfigPath(), args[0])
		return err
	}})
	cmd.AddCommand(group)
	cmd.AddCommand(&cobra.Command{Use: "sync", Short: "Synchronize the next HA generation with peers", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := server.SyncHAGeneration(commandContext(cmd), haServerConfigPath())
		return err
	}})
	peer := &cobra.Command{Use: "peer", Short: "Manage trusted HA peers"}
	allowInsecure := false
	witness := false
	nonVoting := false
	peer.AddCommand(&cobra.Command{Use: "self <id>", Short: "Set the local HA peer identity", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		return server.SaveHALocalPeerID(haServerConfigPath(), args[0])
	}})
	peerAdd := &cobra.Command{Use: "add <id> <endpoint> <secret>", Short: "Add or update an HA peer", Args: cobra.ExactArgs(3), RunE: func(_ *cobra.Command, args []string) error {
		return server.UpsertHAPeer(haServerConfigPath(), ha.Peer{ID: args[0], Endpoint: args[1], Secret: args[2], AllowInsecure: allowInsecure, Witness: witness, NonVoting: nonVoting})
	}}
	peerAdd.Flags().BoolVarP(&allowInsecure, "allow-insecure", "k", false, "allow an untrusted peer certificate")
	peerAdd.Flags().BoolVarP(&witness, "witness", "w", false, "mark peer as a control-plane witness")
	peerAdd.Flags().BoolVarP(&nonVoting, "non-voting", "n", false, "exclude peer from quorum voting")
	peer.AddCommand(peerAdd)
	peer.AddCommand(&cobra.Command{Use: "remove <id>", Short: "Remove an HA peer", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		return server.RemoveHAPeer(haServerConfigPath(), args[0])
	}})
	peer.AddCommand(&cobra.Command{Use: "list", Short: "List HA peers", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		peers, err := server.ListHAPeers(haServerConfigPath())
		if err != nil {
			return err
		}
		if clioutput.Enabled(cmd) {
			type peerResult struct {
				ID            string `json:"id"`
				Endpoint      string `json:"endpoint"`
				AllowInsecure bool   `json:"allow_insecure"`
				Witness       bool   `json:"witness"`
				NonVoting     bool   `json:"non_voting"`
			}
			result := struct {
				Peers []peerResult `json:"peers"`
			}{Peers: make([]peerResult, 0, len(peers))}
			for _, item := range peers {
				result.Peers = append(result.Peers, peerResult{item.ID, item.Endpoint, item.AllowInsecure, item.Witness, item.NonVoting})
			}
			return clioutput.SetResult(cmd, result)
		}
		for _, item := range peers {
			fmt.Printf("%s\t%s\n", item.ID, item.Endpoint)
		}
		return nil
	}})
	cmd.AddCommand(peer)
	channel := &cobra.Command{Use: "channel", Short: "Manage stable HA reverse channels"}
	channel.AddCommand(&cobra.Command{Use: "create <id> <tag> <domain>", Short: "Create a group-bound HA channel", Args: cobra.ExactArgs(3), RunE: func(_ *cobra.Command, args []string) error {
		generation, err := server.LoadHAGeneration(haServerConfigPath())
		if err != nil {
			return err
		}
		_, err = server.AddHAChannel(haServerConfigPath(), ha.Channel{ID: args[0], Tag: args[1], Domain: args[2], Binding: ha.ChannelBinding{GroupTag: generation.Group.Tag}})
		return err
	}})
	channel.AddCommand(&cobra.Command{Use: "disable <id>", Short: "Disable an HA channel", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.RebindHAChannel(haServerConfigPath(), args[0], "", "", true)
		return err
	}})
	channel.AddCommand(&cobra.Command{Use: "inspect <id>", Short: "Inspect an HA channel", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		generation, err := server.LoadHAGeneration(haServerConfigPath())
		if err != nil {
			return err
		}
		for _, item := range generation.Channels {
			if strings.EqualFold(item.ID, args[0]) {
				if clioutput.Enabled(cmd) {
					return clioutput.SetResult(cmd, item)
				}
				fmt.Printf("ID: %s\nTag: %s\nDomain: %s\nGroup: %s\nEndpoint: %s\nDisabled: %t\n", item.ID, item.Tag, item.Domain, item.Binding.GroupTag, item.Binding.EndpointTag, item.Binding.Disabled)
				return nil
			}
		}
		return fmt.Errorf("HA channel %q is not registered", args[0])
	}})
	channel.AddCommand(&cobra.Command{Use: "rebind <id> <group-tag|endpoint-tag>", Short: "Rebind an HA channel", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		generation, err := server.LoadHAGeneration(haServerConfigPath())
		if err != nil {
			return err
		}
		groupTag, endpointTag := "", args[1]
		if strings.EqualFold(args[1], generation.Group.Tag) {
			groupTag, endpointTag = generation.Group.Tag, ""
		}
		_, err = server.RebindHAChannel(haServerConfigPath(), args[0], groupTag, endpointTag, false)
		return err
	}})
	channel.AddCommand(&cobra.Command{Use: "rebind-endpoint <id> <endpoint-tag>", Short: "Bind an HA channel to a physical endpoint", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.RebindHAChannel(haServerConfigPath(), args[0], "", args[1], false)
		return err
	}})
	channel.AddCommand(&cobra.Command{Use: "finalize <id>", Short: "Finalize a disabled HA channel", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.FinalizeHAChannel(haServerConfigPath(), args[0])
		return err
	}})
	cmd.AddCommand(channel)
	redirectCmd := &cobra.Command{Use: "redirect", Short: "Manage group-owned HA redirect policy"}
	redirectCIDR := ""
	redirectDomain := ""
	redirectAccess := ""
	redirectAllowUsers := []string{}
	redirectAllowGroups := []string{}
	redirectAdd := &cobra.Command{Use: "add <channel-id>", Short: "Add a redirect through a group-bound HA channel", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.AddHARedirect(haServerConfigPath(), args[0], redirectCIDR, redirectDomain, redirect.AccessPolicy{Access: redirectAccess, Users: redirectAllowUsers, Groups: redirectAllowGroups})
		return err
	}}
	redirectAdd.Flags().StringVarP(&redirectCIDR, "cidr", "C", "", "CIDR to redirect")
	redirectAdd.Flags().StringVarP(&redirectDomain, "domain", "d", "", "domain to redirect")
	redirectAdd.Flags().StringVarP(&redirectAccess, "access", "V", "", "access policy: all or restricted")
	redirectAdd.Flags().StringSliceVarP(&redirectAllowUsers, "allow-user", "U", nil, "allowed user label (repeatable)")
	redirectAdd.Flags().StringSliceVarP(&redirectAllowGroups, "allow-group", "G", nil, "allowed provider group ID (repeatable)")
	redirectRemove := &cobra.Command{Use: "remove <channel-id>", Short: "Remove a redirect through a group-bound HA channel", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		_, err := server.RemoveHARedirect(haServerConfigPath(), args[0], redirectCIDR, redirectDomain)
		return err
	}}
	redirectRemove.Flags().StringVarP(&redirectCIDR, "cidr", "C", "", "CIDR mapping to remove")
	redirectRemove.Flags().StringVarP(&redirectDomain, "domain", "d", "", "domain mapping to remove")
	redirectCmd.AddCommand(redirectAdd, redirectRemove, &cobra.Command{Use: "list", Short: "List group-owned HA redirect policy", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		rules, err := server.ListHARedirects(haServerConfigPath())
		if err != nil {
			return err
		}
		if clioutput.Enabled(cmd) {
			return clioutput.SetResult(cmd, struct {
				Redirects []redirect.Rule `json:"redirects"`
			}{Redirects: rules})
		}
		for _, rule := range rules {
			fmt.Printf("%s\t%s\t%s\n", rule.Kind(), rule.Value(), rule.OutboundTag)
		}
		return nil
	}})
	cmd.AddCommand(redirectCmd)
	member := &cobra.Command{Use: "member", Short: "Manage HA group members"}
	forceMember := false
	forceReason := ""
	memberTLSName := ""
	memberTLSPin := ""
	member.AddCommand(&cobra.Command{Use: "remove <id>", Short: "Tombstone an HA member", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		if forceMember {
			_, err := server.ForceTombstoneHAMember(haServerConfigPath(), args[0], forceReason)
			return err
		}
		_, err := server.TombstoneHAMember(haServerConfigPath(), args[0])
		return err
	}})
	member.Commands()[0].Flags().BoolVarP(&forceMember, "force", "f", false, "force an emergency two-voter reconfiguration")
	member.Commands()[0].Flags().StringVarP(&forceReason, "reason", "r", "", "audit reason for emergency force-reconfiguration")
	memberAdd := &cobra.Command{Use: "add <id> <tag> <host> <port> <profile>", Short: "Add a confirmed HA member", Args: cobra.ExactArgs(5), RunE: func(_ *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[3])
		if err != nil {
			return err
		}
		member := ha.Member{ID: args[0], Tag: args[1], Host: args[2], Port: port, Profile: args[4], TLSName: memberTLSName, TLSPin: memberTLSPin, Confirmed: true}
		if forceMember {
			_, err = server.ForceAddHAMember(haServerConfigPath(), member, forceReason)
			return err
		}
		_, err = server.AddHAMember(haServerConfigPath(), member)
		return err
	}}
	memberAdd.Flags().BoolVarP(&forceMember, "force", "f", false, "force an emergency two-voter reconfiguration")
	memberAdd.Flags().StringVarP(&forceReason, "reason", "r", "", "audit reason for emergency force-reconfiguration")
	memberAdd.Flags().StringVarP(&memberTLSName, "tls-server-name", "S", "", "TLS server name advertised for this HA member")
	memberAdd.Flags().StringVarP(&memberTLSPin, "tls-pin", "P", "", "pinned peer certificate SHA256 advertised for this HA member")
	member.AddCommand(memberAdd)
	member.AddCommand(&cobra.Command{Use: "reprioritize <id> <priority>", Short: "Change HA member priority", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		priority, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}
		_, err = server.ReprioritizeHAMember(haServerConfigPath(), args[0], priority)
		return err
	}})
	member.AddCommand(&cobra.Command{Use: "list", Short: "List HA group members", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		generation, err := server.LoadHAGeneration(haServerConfigPath())
		if err != nil {
			return err
		}
		if clioutput.Enabled(cmd) {
			return clioutput.SetResult(cmd, struct {
				Members []ha.Member `json:"members"`
			}{Members: generation.Group.Members})
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
	channel.AddCommand(&cobra.Command{Use: "list", Short: "List HA channels", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		generation, err := server.LoadHAGeneration(haServerConfigPath())
		if err != nil {
			return err
		}
		if clioutput.Enabled(cmd) {
			return clioutput.SetResult(cmd, struct {
				Channels []ha.Channel `json:"channels"`
			}{Channels: generation.Channels})
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
