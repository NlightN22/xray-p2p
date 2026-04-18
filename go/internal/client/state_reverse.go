package client

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/naming"
)

func (s *clientInstallState) ensureReverseChannel(userID, host, endpointTag string) (clientReverseChannel, error) {
	s.normalize()
	user := strings.TrimSpace(userID)
	trimmedHost := strings.TrimSpace(host)
	if user == "" || trimmedHost == "" {
		return clientReverseChannel{}, fmt.Errorf("reverse channels require user and host")
	}
	tag, err := naming.ReverseTag(user, trimmedHost)
	if err != nil {
		return clientReverseChannel{}, err
	}
	channel := clientReverseChannel{
		UserID:      user,
		Host:        trimmedHost,
		Tag:         tag,
		Domain:      tag,
		EndpointTag: endpointTag,
	}
	if existing, ok := s.Reverse[tag]; ok {
		if !strings.EqualFold(existing.UserID, channel.UserID) || !strings.EqualFold(existing.Host, channel.Host) {
			return clientReverseChannel{}, fmt.Errorf("reverse tag %s already assigned to %s@%s", tag, existing.UserID, existing.Host)
		}
		if !strings.EqualFold(existing.EndpointTag, endpointTag) {
			return clientReverseChannel{}, fmt.Errorf("reverse tag %s already routed via %s", tag, existing.EndpointTag)
		}
		return existing, nil
	}
	s.Reverse[tag] = channel
	return channel, nil
}

func (s *clientInstallState) removeReverseChannelsByTag(tag string) {
	if len(s.Reverse) == 0 {
		return
	}
	lower := strings.ToLower(strings.TrimSpace(tag))
	if lower == "" {
		return
	}
	for key, channel := range s.Reverse {
		if strings.ToLower(strings.TrimSpace(channel.EndpointTag)) == lower {
			delete(s.Reverse, key)
		}
	}
}
