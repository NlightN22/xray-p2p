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

// rebindReverseChannel preserves the portal identity. Final deletion is only
// permitted once every Desired redirect and group reference has been removed.
func (s *clientInstallState) rebindReverseChannel(tag, groupTag, endpointTag string, disabled bool) error {
	channel, ok := s.Reverse[tag]
	if !ok {
		return fmt.Errorf("reverse channel %s is not registered", tag)
	}
	if !disabled && strings.TrimSpace(groupTag) == "" && strings.TrimSpace(endpointTag) == "" {
		return fmt.Errorf("reverse channel %s requires a binding or disable", tag)
	}
	channel.GroupTag, channel.EndpointTag, channel.Disabled = strings.TrimSpace(groupTag), strings.TrimSpace(endpointTag), disabled
	s.Reverse[tag] = channel
	return nil
}

func (s *clientInstallState) finalizeReverseChannel(tag string) error {
	channel, ok := s.Reverse[tag]
	if !ok {
		return fmt.Errorf("reverse channel %s is not registered", tag)
	}
	for _, rule := range s.Redirects {
		if strings.EqualFold(strings.TrimSpace(rule.OutboundTag), channel.Tag) {
			return fmt.Errorf("reverse channel %s is referenced by a redirect", tag)
		}
	}
	if !channel.Disabled {
		return fmt.Errorf("reverse channel %s must be disabled before finalization", tag)
	}
	delete(s.Reverse, tag)
	return nil
}
