package client

import (
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
)

func TestSubscriptionCandidateUpdatesEndpointWithoutLosingCredential(t *testing.T) {
	current := clientInstallState{
		Endpoints: []clientEndpointRecord{{
			Hostname: "old.example",
			Address:  "old.example",
			Tag:      "proxy-old",
			Port:     8443,
			User:     "alice",
		}},
	}
	sub := controlplane.Subscription{
		Generation: "next",
		Protocol:   "trojan",
		Host:       "new.example",
		Port:       9443,
		ServerName: "new.example",
		TLS: controlplane.TLSMetadata{
			PinnedPeerCertSHA256: "abc123",
			VerifyPeerCertByName: "new.example",
		},
	}
	candidate, err := subscriptionCandidate(current, current.Endpoints[0], sub, "secret")
	if err != nil {
		t.Fatalf("subscriptionCandidate: %v", err)
	}
	got := candidate.Endpoints[0]
	if got.Hostname != "new.example" || got.Port != 9443 {
		t.Fatalf("endpoint not updated: %+v", got)
	}
	if got.Password != "secret" {
		t.Fatalf("credential lost: %+v", got)
	}
	if got.AllowInsecure {
		t.Fatalf("pinned endpoint must not allow insecure TLS")
	}
}

func TestSubscriptionCandidateRejectsMissingCredential(t *testing.T) {
	current := clientInstallState{Endpoints: []clientEndpointRecord{{Tag: "proxy-old", User: "alice"}}}
	_, err := subscriptionCandidate(current, current.Endpoints[0], controlplane.Subscription{
		Protocol: "trojan",
		Host:     "new.example",
		Port:     9443,
	}, "")
	if err == nil {
		t.Fatalf("expected missing credential error")
	}
}

func TestSubscriptionCandidateSwitchesToVLESSProfile(t *testing.T) {
	current := clientInstallState{Endpoints: []clientEndpointRecord{{
		Profile: "trojan-tls", Protocol: "trojan", Transport: "tcp", Security: "tls",
		Hostname: "edge.example", Address: "192.0.2.10", Tag: "proxy-edge", Port: 443, User: "alice",
	}}}
	sub := controlplane.Subscription{
		Generation: "vless-generation", Profile: "vless-tls-vision", Protocol: "vless", Transport: "tcp", Security: "tls",
		Host: "edge.example", Port: 443, ServerName: "edge.example", Parameters: map[string]string{"flow": "xtls-rprx-vision"},
	}
	candidate, err := subscriptionCandidate(current, current.Endpoints[0], sub, "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("subscriptionCandidate: %v", err)
	}
	got := candidate.Endpoints[0]
	if got.Profile != "vless-tls-vision" || got.Protocol != "vless" || got.Flow != "xtls-rprx-vision" {
		t.Fatalf("profile was not applied: %+v", got)
	}
	if got.Address != "192.0.2.10" {
		t.Fatalf("resolved endpoint address was not preserved: %+v", got)
	}
}
