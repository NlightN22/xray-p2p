package client

import (
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/ha"
)

func TestSubscriptionTopologyCreatesConfirmedGroupMembers(t *testing.T) {
	state := clientInstallState{Endpoints: []clientEndpointRecord{{Tag: "primary", User: "alice", Profile: "trojan-tls"}}}
	sub := controlplane.Subscription{Topology: &controlplane.Topology{Generation: 3, Group: ha.Group{ID: "g", Tag: "logical", Selector: ha.Selector{Mode: "automatic"}, Members: []ha.Member{{ID: "one", Tag: "primary", Host: "one.example", Port: 443, Profile: "trojan-tls", Confirmed: true}, {ID: "old", Tag: "old", Tombstone: true, Confirmed: true}}}}}
	updated, err := applySubscriptionTopology(state, state.Endpoints[0], sub, "secret")
	if err != nil { t.Fatal(err) }
	if len(updated.Endpoints) != 1 || updated.Endpoints[0].Hostname != "one.example" { t.Fatalf("endpoints = %+v", updated.Endpoints) }
	if len(updated.EndpointGroups) != 1 || updated.EndpointGroups[0].Tag != "logical" { t.Fatalf("groups = %+v", updated.EndpointGroups) }
}
