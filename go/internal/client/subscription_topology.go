package client

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/ha"
)

func applySubscriptionTopology(current clientInstallState, endpoint clientEndpointRecord, sub controlplane.Subscription, secret string) (clientInstallState, error) {
	if sub.Topology == nil {
		return current, nil
	}
	topology := sub.Topology
	if topology.Generation == 0 || strings.TrimSpace(topology.Group.ID) == "" || strings.TrimSpace(topology.Group.Tag) == "" {
		return clientInstallState{}, fmt.Errorf("subscription HA topology is incomplete")
	}
	updated := current
	byTag := make(map[string]clientEndpointRecord, len(current.Endpoints))
	for _, record := range current.Endpoints {
		byTag[strings.ToLower(record.Tag)] = record
	}
	endpoints := make([]clientEndpointRecord, 0, len(topology.Group.Members))
	members := make([]string, 0, len(topology.Group.Members))
	for _, member := range confirmedTopologyMembers(topology.Group.Members) {
		record, ok := byTag[strings.ToLower(member.Tag)]
		if !ok {
			record = endpoint
			record.Tag = member.Tag
		}
		if strings.TrimSpace(member.Host) == "" || member.Port <= 0 || member.Port > 65535 {
			return clientInstallState{}, fmt.Errorf("subscription HA member %q is invalid", member.ID)
		}
		record.Hostname, record.Address, record.Port = member.Host, member.Host, member.Port
		record.Profile, record.Password = member.Profile, secret
		if member.TLSName != "" {
			record.ServerName = member.TLSName
		}
		if member.TLSPin != "" {
			record.PinnedPeerCertSHA256, record.AllowInsecure = member.TLSPin, false
		}
		endpoints = append(endpoints, record)
		members = append(members, record.Tag)
	}
	if len(endpoints) == 0 {
		return clientInstallState{}, fmt.Errorf("subscription HA topology has no confirmed members")
	}
	updated.Endpoints = endpoints
	group := endpointGroup{GroupID: topology.Group.ID, Tag: topology.Group.Tag, Members: members, Mode: endpointGroupMode(topology.Group.Selector.Mode), FailureThreshold: topology.Group.Selector.FailureThreshold, SuccessThreshold: topology.Group.Selector.SuccessThreshold, CooldownSeconds: topology.Group.Selector.CooldownSeconds, MinimumHoldSeconds: topology.Group.Selector.MinimumHoldSeconds, AutomaticFailback: topology.Group.Selector.AutomaticFailback}
	updated.EndpointGroups = []endpointGroup{group}
	updated.normalize()
	if err := updated.validateEndpointGroups(); err != nil {
		return clientInstallState{}, err
	}
	return updated, nil
}

func confirmedTopologyMembers(members []ha.Member) []ha.Member {
	confirmed := make([]ha.Member, 0, len(members))
	for _, member := range members {
		if member.Confirmed && !member.Tombstone {
			confirmed = append(confirmed, member)
		}
	}
	return confirmed
}
