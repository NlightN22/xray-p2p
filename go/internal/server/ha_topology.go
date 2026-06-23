package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/ha"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

const (
	serverHAGenerationKey = "ha_generation"
	serverHAPeersKey      = "ha_peers"
)

func LoadHAReplication(configPath string) (*ha.Store, error) {
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return nil, err
	}
	generation, err := decodeHAGeneration(doc)
	if err != nil {
		return nil, err
	}
	var peers []ha.Peer
	if raw := doc[serverHAPeersKey]; raw != nil {
		data, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &peers); err != nil {
			return nil, fmt.Errorf("parse HA peers: %w", err)
		}
	}
	store, err := ha.NewStore(peers, generation)
	if err != nil {
		return nil, err
	}
	store.SetCommitter(func(candidate ha.Generation) error { return CommitHAGeneration(configPath, candidate) })
	return store, nil
}

func decodeHAGeneration(doc map[string]any) (ha.Generation, error) {
	raw := doc[serverHAGenerationKey]
	if raw == nil {
		return ha.Generation{}, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return ha.Generation{}, fmt.Errorf("encode HA generation: %w", err)
	}
	var generation ha.Generation
	if err := json.Unmarshal(data, &generation); err != nil {
		return ha.Generation{}, fmt.Errorf("parse HA generation: %w", err)
	}
	if err := generation.Validate(); err != nil {
		return ha.Generation{}, err
	}
	return generation, nil
}

// CommitHAGeneration persists only HA-owned resources. Node-local settings and
// unrelated user-owned reverse channels remain intact.
func CommitHAGeneration(configPath string, generation ha.Generation) error {
	if err := generation.Validate(); err != nil {
		return err
	}
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return err
	}
	previous, err := decodeHAGeneration(doc)
	if err != nil {
		return err
	}
	if generation.Number <= previous.Number {
		return ha.ErrGenerationOutOfOrder
	}
	reverse, err := decodeServerReverseState(doc)
	if err != nil {
		return err
	}
	owned := make(map[string]struct{}, len(previous.Channels))
	for _, channel := range previous.Channels {
		owned[strings.ToLower(channel.Tag)] = struct{}{}
	}
	for tag := range owned {
		delete(reverse, tag)
	}
	for _, channel := range generation.Channels {
		if channel.Binding.Disabled || !strings.EqualFold(channel.Binding.GroupTag, generation.Group.Tag) {
			continue
		}
		reverse[channel.Tag] = serverReverseChannel{UserID: channel.UserID, Tag: channel.Tag, Domain: channel.Domain, Host: channel.Domain}
	}
	doc[serverReverseStateKey] = reverse
	if len(generation.Redirects) > 0 {
		var redirects []redirect.Rule
		if err := json.Unmarshal(generation.Redirects, &redirects); err != nil {
			return fmt.Errorf("parse HA redirects: %w", err)
		}
		doc[serverRedirectRulesKey] = redirects
	}
	doc[serverHAGenerationKey] = generation
	return writeServerStateDoc(configPath, doc)
}

func LoadHAGeneration(configPath string) (ha.Generation, error) {
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return ha.Generation{}, err
	}
	return decodeHAGeneration(doc)
}

func SaveHAPeers(configPath string, peers []ha.Peer) error {
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return err
	}
	if len(peers) == 0 {
		doc[serverHAPeersKey] = nil
	} else {
		doc[serverHAPeersKey] = peers
	}
	return writeServerStateDoc(configPath, doc)
}

func UpsertHAPeer(configPath string, peer ha.Peer) error {
	if strings.TrimSpace(peer.ID) == "" || strings.TrimSpace(peer.Secret) == "" {
		return fmt.Errorf("HA peer ID and secret are required")
	}
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return err
	}
	var peers []ha.Peer
	if raw := doc[serverHAPeersKey]; raw != nil {
		data, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &peers); err != nil {
			return fmt.Errorf("parse HA peers: %w", err)
		}
	}
	found := false
	for i := range peers {
		if strings.EqualFold(peers[i].ID, peer.ID) {
			peers[i] = peer
			found = true
		}
	}
	if !found {
		peers = append(peers, peer)
	}
	doc[serverHAPeersKey] = peers
	return writeServerStateDoc(configPath, doc)
}

func RemoveHAPeer(configPath, id string) error {
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return err
	}
	var peers []ha.Peer
	if raw := doc[serverHAPeersKey]; raw != nil {
		data, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &peers); err != nil {
			return fmt.Errorf("parse HA peers: %w", err)
		}
	}
	filtered := peers[:0]
	for _, peer := range peers {
		if !strings.EqualFold(peer.ID, strings.TrimSpace(id)) {
			filtered = append(filtered, peer)
		}
	}
	doc[serverHAPeersKey] = filtered
	return writeServerStateDoc(configPath, doc)
}
