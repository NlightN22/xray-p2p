package ha

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrGenerationOutOfOrder = errors.New("HA generation is not newer than the committed generation")
	ErrMemberNotConfirmed   = errors.New("HA member is not confirmed")
	ErrChannelReferenced    = errors.New("HA channel is still referenced")
)

type Member struct {
	ID        string `json:"id" toml:"id"`
	Tag       string `json:"tag" toml:"tag"`
	Host      string `json:"host" toml:"host"`
	Port      int    `json:"port" toml:"port"`
	Profile   string `json:"profile" toml:"profile"`
	TLSName   string `json:"tls_server_name,omitempty" toml:"tls_server_name,omitempty"`
	TLSPin    string `json:"tls_pin,omitempty" toml:"tls_pin,omitempty"`
	Priority  int    `json:"priority" toml:"priority"`
	Confirmed bool   `json:"confirmed" toml:"confirmed"`
	Tombstone bool   `json:"tombstone,omitempty" toml:"tombstone,omitempty"`
}

type ChannelBinding struct {
	GroupTag    string `json:"group_tag,omitempty" toml:"group_tag,omitempty"`
	EndpointTag string `json:"endpoint_tag,omitempty" toml:"endpoint_tag,omitempty"`
	Disabled    bool   `json:"disabled,omitempty" toml:"disabled,omitempty"`
}

type Channel struct {
	ID      string         `json:"id" toml:"id"`
	Tag     string         `json:"tag" toml:"tag"`
	Domain  string         `json:"domain" toml:"domain"`
	UserID  string         `json:"user_id,omitempty" toml:"user_id,omitempty"`
	Binding ChannelBinding `json:"binding" toml:"binding"`
}

type Group struct {
	ID       string   `json:"id" toml:"id"`
	Tag      string   `json:"tag" toml:"tag"`
	Members  []Member `json:"members" toml:"members"`
	Selector Selector `json:"selector" toml:"selector"`
}

type Selector struct {
	Mode               string `json:"mode" toml:"mode"`
	FailureThreshold   int    `json:"failure_threshold" toml:"failure_threshold"`
	SuccessThreshold   int    `json:"success_threshold" toml:"success_threshold"`
	CooldownSeconds    int    `json:"cooldown_seconds" toml:"cooldown_seconds"`
	MinimumHoldSeconds int    `json:"minimum_hold_seconds" toml:"minimum_hold_seconds"`
	AutomaticFailback  bool   `json:"automatic_failback" toml:"automatic_failback"`
}

type Generation struct {
	Number      uint64    `json:"number" toml:"number"`
	Group       Group     `json:"group" toml:"group"`
	Channels    []Channel `json:"channels" toml:"channels"`
	Redirects   []byte    `json:"redirects,omitempty" toml:"redirects,omitempty"`
	IdentityACL []byte    `json:"identity_acl,omitempty" toml:"identity_acl,omitempty"`
	Provisioned []byte    `json:"provisioned_resources,omitempty" toml:"provisioned_resources,omitempty"`
	Tombstones  []string  `json:"tombstones,omitempty" toml:"tombstones,omitempty"`
}

func (g Generation) Validate() error {
	if strings.TrimSpace(g.Group.ID) == "" || strings.TrimSpace(g.Group.Tag) == "" {
		return errors.New("HA group ID and tag are required")
	}
	seen := make(map[string]struct{}, len(g.Group.Members))
	for _, member := range g.Group.Members {
		key := strings.ToLower(strings.TrimSpace(member.ID))
		if key == "" || strings.TrimSpace(member.Tag) == "" {
			return errors.New("HA member ID and tag are required")
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate HA member %q", member.ID)
		}
		seen[key] = struct{}{}
	}
	channels := make(map[string]struct{}, len(g.Channels))
	for _, channel := range g.Channels {
		if strings.TrimSpace(channel.ID) == "" || strings.TrimSpace(channel.Tag) == "" || strings.TrimSpace(channel.Domain) == "" {
			return errors.New("HA channel ID, tag, and domain are required")
		}
		if _, ok := channels[strings.ToLower(channel.ID)]; ok {
			return fmt.Errorf("duplicate HA channel %q", channel.ID)
		}
		channels[strings.ToLower(channel.ID)] = struct{}{}
		bound := strings.TrimSpace(channel.Binding.GroupTag) != "" || strings.TrimSpace(channel.Binding.EndpointTag) != ""
		if !channel.Binding.Disabled && !bound {
			return fmt.Errorf("HA channel %q is not bound or disabled", channel.ID)
		}
		if strings.TrimSpace(channel.Binding.GroupTag) != "" && !strings.EqualFold(channel.Binding.GroupTag, g.Group.Tag) {
			return fmt.Errorf("HA channel %q belongs to another group", channel.ID)
		}
	}
	return nil
}

func (g Generation) ConfirmedMembers() []Member {
	members := make([]Member, 0, len(g.Group.Members))
	for _, member := range g.Group.Members {
		if member.Confirmed && !member.Tombstone {
			members = append(members, member)
		}
	}
	sort.SliceStable(members, func(i, j int) bool { return members[i].Priority < members[j].Priority })
	return members
}

func (g Generation) CanFinalizeChannel(id string) error {
	for _, channel := range g.Channels {
		if strings.EqualFold(channel.ID, id) && !channel.Binding.Disabled {
			return ErrChannelReferenced
		}
	}
	return nil
}
