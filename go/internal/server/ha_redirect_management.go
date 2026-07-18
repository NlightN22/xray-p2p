package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/ha"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

// AddHARedirect adds a group-owned redirect bound to an enabled stable channel.
func AddHARedirect(configPath, channelID, cidr, domain string, access redirect.AccessPolicy) (ha.Generation, error) {
	target, err := redirect.ResolveRule(cidr, domain)
	if err != nil {
		return ha.Generation{}, err
	}
	policy, err := access.Normalized()
	if err != nil {
		return ha.Generation{}, err
	}
	return MutateHAGeneration(configPath, func(generation *ha.Generation) error {
		channel, err := groupRedirectChannel(*generation, channelID)
		if err != nil {
			return err
		}
		rules, err := decodeHARedirectPayload(generation.Redirects)
		if err != nil {
			return err
		}
		rule := redirect.Rule{OutboundTag: channel.Tag}
		if target.Kind == redirect.KindDomain {
			rule.Domain = target.Value
		} else {
			rule.CIDR = target.Value
		}
		rule.AccessPolicy = policy
		updated, err := redirect.AddRule(rules, rule)
		if err != nil {
			return err
		}
		generation.Redirects, err = encodeHARedirectPayload(updated)
		return err
	})
}

// RemoveHARedirect removes one group-owned redirect from its stable channel.
func RemoveHARedirect(configPath, channelID, cidr, domain string) (ha.Generation, error) {
	target, err := redirect.ResolveRule(cidr, domain)
	if err != nil {
		return ha.Generation{}, err
	}
	return MutateHAGeneration(configPath, func(generation *ha.Generation) error {
		channel, err := groupRedirectChannel(*generation, channelID)
		if err != nil {
			return err
		}
		rules, err := decodeHARedirectPayload(generation.Redirects)
		if err != nil {
			return err
		}
		updated, removed := redirect.RemoveRule(rules, target, channel.Tag)
		if !removed {
			return fmt.Errorf("HA redirect %s via channel %q is not registered", target.Describe(), channelID)
		}
		generation.Redirects, err = encodeHARedirectPayload(updated)
		return err
	})
}

func ListHARedirects(configPath string) ([]redirect.Rule, error) {
	generation, err := LoadHAGeneration(configPath)
	if err != nil {
		return nil, err
	}
	return decodeHARedirectPayload(generation.Redirects)
}

func groupRedirectChannel(generation ha.Generation, channelID string) (ha.Channel, error) {
	for _, channel := range generation.Channels {
		if !strings.EqualFold(channel.ID, strings.TrimSpace(channelID)) {
			continue
		}
		if channel.Binding.Disabled || !strings.EqualFold(channel.Binding.GroupTag, generation.Group.Tag) {
			return ha.Channel{}, fmt.Errorf("HA channel %q is not bound to group %q", channelID, generation.Group.Tag)
		}
		return channel, nil
	}
	return ha.Channel{}, fmt.Errorf("HA channel %q is not registered", channelID)
}

func encodeHARedirectPayload(rules []redirect.Rule) ([]byte, error) {
	if len(rules) == 0 {
		return nil, nil
	}
	return json.Marshal(rules)
}
