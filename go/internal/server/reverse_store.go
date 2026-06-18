//go:build windows || linux

package server

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/naming"
)

type reverseStore struct {
	path  string
	doc   map[string]any
	state serverReverseState
}

func openReverseStore(installDir string) (reverseStore, error) {
	path := serverStatePath(installDir)
	doc, err := loadServerStateDoc(path)
	if err != nil {
		return reverseStore{}, err
	}
	state, err := decodeServerReverseState(doc)
	if err != nil {
		return reverseStore{}, err
	}
	return reverseStore{
		path:  path,
		doc:   doc,
		state: state,
	}, nil
}

func (s *reverseStore) ensureAvailable(channel serverReverseChannel) error {
	existing, ok := s.state[channel.Tag]
	if !ok {
		return nil
	}
	if strings.EqualFold(existing.UserID, channel.UserID) {
		return nil
	}
	return fmt.Errorf("reverse tag %s already assigned to %s", channel.Tag, existing.UserID)
}

func (s *reverseStore) put(channel serverReverseChannel) {
	s.state.ensure()
	s.state[channel.Tag] = channel
}

func (s *reverseStore) delete(tag string) {
	if s.state == nil {
		return
	}
	delete(s.state, tag)
}

func (s *reverseStore) deleteByUser(userID string) []serverReverseChannel {
	if s.state == nil {
		return nil
	}
	trimmedUser := strings.TrimSpace(userID)
	if trimmedUser == "" {
		return nil
	}
	removed := make([]serverReverseChannel, 0)
	for tag, channel := range s.state {
		if !strings.EqualFold(strings.TrimSpace(channel.UserID), trimmedUser) {
			continue
		}
		removed = append(removed, channel)
		delete(s.state, tag)
	}
	return removed
}

func (s *reverseStore) save() error {
	if len(s.state) == 0 {
		s.doc[serverReverseStateKey] = nil
	} else {
		s.doc[serverReverseStateKey] = s.state
	}
	return writeServerStateDoc(s.path, s.doc)
}

func buildServerReverseChannel(userID, host string) (serverReverseChannel, error) {
	trimmed := strings.TrimSpace(userID)
	if trimmed == "" {
		return serverReverseChannel{}, errUserIDRequired
	}
	hostValue := strings.TrimSpace(host)
	tag, err := naming.ReverseTag(trimmed, hostValue)
	if err != nil {
		return serverReverseChannel{}, err
	}
	return serverReverseChannel{
		UserID: trimmed,
		Host:   hostValue,
		Tag:    tag,
		Domain: tag,
	}, nil
}

func applyServerReverseChannel(store *reverseStore, installDir string, configDir string, channel serverReverseChannel) error {
	configPath := serverStatePath(installDir)
	return applyServerReverseChannelWithConfig(store, configPath, configDir, channel)
}

func purgeServerReverseChannel(store *reverseStore, installDir string, configDir string, channel serverReverseChannel) error {
	configPath := serverStatePath(installDir)
	return purgeServerReverseChannelWithConfig(store, configPath, configDir, channel)
}

func applyServerReverseChannelWithConfig(store *reverseStore, configPath string, configDir string, channel serverReverseChannel) error {
	store.put(channel)
	if err := store.save(); err != nil {
		return err
	}
	_ = configPath
	_ = configDir
	return nil
}

func purgeServerReverseChannelWithConfig(store *reverseStore, configPath string, configDir string, channel serverReverseChannel) error {
	store.delete(channel.Tag)
	if err := store.save(); err != nil {
		return err
	}
	_ = configPath
	_ = configDir
	return nil
}
