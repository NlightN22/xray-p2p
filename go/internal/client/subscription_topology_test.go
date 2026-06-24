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
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Endpoints) != 1 || updated.Endpoints[0].Hostname != "one.example" {
		t.Fatalf("endpoints = %+v", updated.Endpoints)
	}
	if len(updated.EndpointGroups) != 1 || updated.EndpointGroups[0].Tag != "logical" {
		t.Fatalf("groups = %+v", updated.EndpointGroups)
	}
}

func TestSubscriptionTopologyPreservesUnrelatedEndpointsAndGroups(t *testing.T) {
	state := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{Tag: "primary", User: "alice", Profile: "trojan-tls"},
			{Tag: "single", Hostname: "single.example", Address: "single.example", Port: 443, User: "bob"},
		},
		EndpointGroups: []endpointGroup{{GroupID: "manual", Tag: "manual", Members: []string{"single"}, Mode: endpointGroupMode("manual")}},
	}
	sub := controlplane.Subscription{Topology: &controlplane.Topology{
		Generation: 4,
		Group: ha.Group{
			ID:       "ha",
			Tag:      "ha",
			Selector: ha.Selector{Mode: "automatic"},
			Members:  []ha.Member{{ID: "one", Tag: "primary", Host: "one.example", Port: 443, Profile: "trojan-tls", Confirmed: true}},
		},
	}}
	updated, err := applySubscriptionTopology(state, state.Endpoints[0], sub, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Endpoints) != 2 {
		t.Fatalf("endpoints = %+v", updated.Endpoints)
	}
	if updated.Endpoints[0].Tag != "single" || updated.Endpoints[0].Hostname != "single.example" {
		t.Fatalf("unrelated endpoint was not preserved: %+v", updated.Endpoints)
	}
	if len(updated.EndpointGroups) != 2 {
		t.Fatalf("groups = %+v", updated.EndpointGroups)
	}
	if updated.EndpointGroups[0].Tag != "manual" || updated.EndpointGroups[1].Tag != "ha" {
		t.Fatalf("groups = %+v", updated.EndpointGroups)
	}
}

func TestSubscriptionTopologyAppliesMemberTLSMetadata(t *testing.T) {
	state := clientInstallState{Endpoints: []clientEndpointRecord{{
		Tag:                  "primary",
		User:                 "u",
		ServerName:           "primary.example",
		VerifyPeerCertByName: "primary.example",
		PinnedPeerCertSHA256: "primary-pin",
	}}}
	sub := controlplane.Subscription{Topology: &controlplane.Topology{
		Generation: 1,
		Group: ha.Group{
			ID:       "g",
			Tag:      "ha",
			Selector: ha.Selector{Mode: "automatic"},
			Members: []ha.Member{{
				ID:        "backup",
				Tag:       "backup",
				Host:      "backup.example",
				Port:      443,
				Profile:   "trojan-tls",
				TLSName:   "backup.example",
				TLSPin:    "backup-pin",
				Confirmed: true,
			}},
		},
	}}
	updated, err := applySubscriptionTopology(state, state.Endpoints[0], sub, "secret")
	if err != nil {
		t.Fatalf("applySubscriptionTopology: %v", err)
	}
	endpoint := clientEndpointRecord{}
	for _, item := range updated.Endpoints {
		if item.Tag == "backup" {
			endpoint = item
			break
		}
	}
	if endpoint.ServerName != "backup.example" || endpoint.VerifyPeerCertByName != "backup.example" || endpoint.PinnedPeerCertSHA256 != "backup-pin" {
		t.Fatalf("backup TLS metadata = %+v", endpoint)
	}
}
