package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/ha"
)

const (
	serverHAGenerationKey   = "ha_generation"
	serverHALocalPeerIDKey  = "ha_local_peer_id"
	serverHAPeersKey        = "ha_peers"
	serverHAIdentityACLKey  = "ha_identity_acl"
	serverHAProvisionedKey  = "ha_provisioned_resources"
	serverHARedirectKeysKey = "ha_redirect_keys"
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
	localPeerID, _ := doc[serverHALocalPeerIDKey].(string)
	store, err := ha.NewStoreWithLocalID(localPeerID, peers, generation)
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
		if channel.Binding.Disabled {
			continue
		}
		reverse[channel.Tag] = serverReverseChannel{UserID: channel.UserID, Tag: channel.Tag, Domain: channel.Domain, Host: channel.Domain}
	}
	doc[serverReverseStateKey] = reverse
	redirects, err := mergeHAOwnedRedirects(doc, generation.Redirects)
	if err != nil {
		return err
	}
	doc[serverRedirectRulesKey] = redirects
	doc[serverHARedirectKeysKey] = redirectKeysFromPayload(generation.Redirects)
	doc[serverHAGenerationKey] = generation
	if len(generation.IdentityACL) == 0 {
		doc[serverHAIdentityACLKey] = nil
	} else {
		doc[serverHAIdentityACLKey] = string(generation.IdentityACL)
	}
	if len(generation.Provisioned) == 0 {
		doc[serverHAProvisionedKey] = nil
	} else {
		doc[serverHAProvisionedKey] = string(generation.Provisioned)
	}
	if filepath.Clean(configPath) == filepath.Clean(pendingConfigPath()) {
		if err := commitServerRuntimeDoc(context.Background(), doc); err != nil {
			return err
		}
		return applyHAIdentityState(generation.IdentityACL, generation.Provisioned)
	}
	if err := writeServerStateDoc(configPath, doc); err != nil {
		return err
	}
	return applyHAIdentityState(generation.IdentityACL, generation.Provisioned)
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

func SaveHALocalPeerID(configPath, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("HA local peer ID is required")
	}
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return err
	}
	doc[serverHALocalPeerIDKey] = strings.TrimSpace(id)
	return writeServerStateDoc(configPath, doc)
}

func LoadHALocalPeerID(configPath string) (string, error) {
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return "", err
	}
	value, _ := doc[serverHALocalPeerIDKey].(string)
	return strings.TrimSpace(value), nil
}

func ListHAPeers(configPath string) ([]ha.Peer, error) {
	doc, err := loadServerStateDoc(configPath)
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
	return peers, nil
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
