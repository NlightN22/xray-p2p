package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/ha"
	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

func MutateHAGeneration(configPath string, mutate func(*ha.Generation) error) (ha.Generation, error) {
	current, err := LoadHAGeneration(configPath)
	if err != nil {
		return ha.Generation{}, err
	}
	if current.Number == 0 {
		return ha.Generation{}, errors.New("HA generation is not initialized")
	}
	candidate := current
	candidate.Number++
	if err := mutate(&candidate); err != nil {
		return ha.Generation{}, err
	}
	if err := attachHAIdentitySnapshot(&candidate); err != nil {
		return ha.Generation{}, err
	}
	if err := commitHACandidate(context.Background(), configPath, candidate); err != nil {
		return ha.Generation{}, err
	}
	return LoadHAGeneration(configPath)
}

func ForceMutateHAGeneration(configPath, reason string, mutate func(*ha.Generation) error) (ha.Generation, error) {
	current, err := LoadHAGeneration(configPath)
	if err != nil {
		return ha.Generation{}, err
	}
	if current.Number == 0 {
		return ha.Generation{}, errors.New("HA generation is not initialized")
	}
	candidate := current
	candidate.Number++
	if err := mutate(&candidate); err != nil {
		return ha.Generation{}, err
	}
	if err := attachHAIdentitySnapshot(&candidate); err != nil {
		return ha.Generation{}, err
	}
	store, err := LoadHAReplication(configPath)
	if err != nil {
		return ha.Generation{}, err
	}
	if _, err := store.ForceReconfigure(candidate, ha.ForceReconfiguration{Authorization: "force", Reason: reason}); err != nil {
		return ha.Generation{}, err
	}
	return LoadHAGeneration(configPath)
}

func InitializeHAGroup(configPath string, group ha.Group) (ha.Generation, error) {
	current, err := LoadHAGeneration(configPath)
	if err != nil {
		return ha.Generation{}, err
	}
	if current.Number != 0 {
		return ha.Generation{}, errors.New("HA group is already initialized")
	}
	if err := commitHACandidate(context.Background(), configPath, ha.Generation{Number: 1, Group: group, Channels: []ha.Channel{}}); err != nil {
		return ha.Generation{}, err
	}
	return LoadHAGeneration(configPath)
}

func AddHAMember(configPath string, member ha.Member) (ha.Generation, error) {
	return MutateHAGeneration(configPath, func(generation *ha.Generation) error {
		generation.Group.Members = append(generation.Group.Members, member)
		return nil
	})
}

func ForceAddHAMember(configPath string, member ha.Member, reason string) (ha.Generation, error) {
	return ForceMutateHAGeneration(configPath, reason, func(generation *ha.Generation) error {
		generation.Group.Members = append(generation.Group.Members, member)
		return nil
	})
}

func AddHAChannel(configPath string, channel ha.Channel) (ha.Generation, error) {
	return MutateHAGeneration(configPath, func(generation *ha.Generation) error {
		generation.Channels = append(generation.Channels, channel)
		return nil
	})
}

func ReprioritizeHAMember(configPath, id string, priority int) (ha.Generation, error) {
	return MutateHAGeneration(configPath, func(generation *ha.Generation) error {
		for i := range generation.Group.Members {
			if strings.EqualFold(generation.Group.Members[i].ID, id) {
				generation.Group.Members[i].Priority = priority
				return nil
			}
		}
		return fmt.Errorf("HA member %q is not registered", id)
	})
}

func RebindHAChannel(configPath, id, groupTag, endpointTag string, disabled bool) (ha.Generation, error) {
	return MutateHAGeneration(configPath, func(generation *ha.Generation) error {
		for i := range generation.Channels {
			if strings.EqualFold(generation.Channels[i].ID, id) {
				generation.Channels[i].Binding = ha.ChannelBinding{GroupTag: groupTag, EndpointTag: endpointTag, Disabled: disabled}
				return nil
			}
		}
		return fmt.Errorf("HA channel %q is not registered", id)
	})
}

func FinalizeHAChannel(configPath, id string) (ha.Generation, error) {
	return MutateHAGeneration(configPath, func(generation *ha.Generation) error {
		if err := generation.CanFinalizeChannel(id); err != nil {
			return err
		}
		channels := generation.Channels[:0]
		for _, channel := range generation.Channels {
			if !strings.EqualFold(channel.ID, id) {
				channels = append(channels, channel)
			}
		}
		generation.Channels = channels
		return nil
	})
}

func TombstoneHAMember(configPath, id string) (ha.Generation, error) {
	return MutateHAGeneration(configPath, func(generation *ha.Generation) error {
		for i := range generation.Group.Members {
			if strings.EqualFold(generation.Group.Members[i].ID, id) {
				generation.Group.Members[i].Tombstone = true
				generation.Group.Members[i].Confirmed = false
				generation.Tombstones = append(generation.Tombstones, id)
				return nil
			}
		}
		return fmt.Errorf("HA member %q is not registered", id)
	})
}

func ForceTombstoneHAMember(configPath, id, reason string) (ha.Generation, error) {
	return ForceMutateHAGeneration(configPath, reason, func(generation *ha.Generation) error {
		for i := range generation.Group.Members {
			if strings.EqualFold(generation.Group.Members[i].ID, id) {
				generation.Group.Members[i].Tombstone = true
				generation.Group.Members[i].Confirmed = false
				generation.Tombstones = append(generation.Tombstones, id)
				return nil
			}
		}
		return fmt.Errorf("HA member %q is not registered", id)
	})
}

func RemoveHAGroup(configPath string) (ha.Generation, error) {
	return MutateHAGeneration(configPath, func(generation *ha.Generation) error {
		for _, channel := range generation.Channels {
			if strings.EqualFold(channel.Binding.GroupTag, generation.Group.Tag) && !channel.Binding.Disabled {
				return fmt.Errorf("HA channel %q must be rebound or disabled before group removal", channel.ID)
			}
		}
		generation.Group.Members = nil
		generation.Tombstones = append(generation.Tombstones, generation.Group.ID)
		return nil
	})
}

func SetHAGroupMode(configPath, mode string) (ha.Generation, error) {
	return MutateHAGeneration(configPath, func(generation *ha.Generation) error {
		generation.Group.Selector.Mode = strings.ToLower(strings.TrimSpace(mode))
		return generation.Group.Selector.Validate()
	})
}

func SyncHAGeneration(ctx context.Context, configPath string) (ha.Generation, error) {
	candidate, err := LoadHAGeneration(configPath)
	if err != nil {
		return ha.Generation{}, err
	}
	if candidate.Number == 0 {
		return ha.Generation{}, errors.New("HA generation is not initialized")
	}
	candidate.Number++
	if err := attachHAIdentitySnapshot(&candidate); err != nil {
		return ha.Generation{}, err
	}
	if err := commitHACandidate(ctx, configPath, candidate); err != nil {
		return ha.Generation{}, err
	}
	return LoadHAGeneration(configPath)
}

func commitHACandidate(ctx context.Context, configPath string, candidate ha.Generation) error {
	store, err := LoadHAReplication(configPath)
	if err != nil {
		return err
	}
	secureClient := ownedhttp.NewClient(ownedhttp.ClientOptions{})
	insecureClient := ownedhttp.NewClient(ownedhttp.ClientOptions{
		TLSConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	})
	defer shutdownHAClient(secureClient)
	defer shutdownHAClient(insecureClient)
	coordinator := ha.Coordinator{Client: ha.SyncClient{HTTPClientForPeer: func(peer ha.Peer) ownedhttp.Doer {
		if peer.AllowInsecure {
			return insecureClient
		}
		return secureClient
	}}}
	return coordinator.Sync(ctx, store, candidate)
}

func shutdownHAClient(client ownedhttp.OwnedClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = client.Shutdown(ctx)
}
