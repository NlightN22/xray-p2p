//go:build linux

package dnsforward

import (
	"net/netip"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
)

func TestMatchingForwardsRequiresTargetAndUDP(t *testing.T) {
	addr := netip.MustParseAddr("10.50.10.1")
	forwards := []forward.Rule{
		{
			ListenAddress: "127.0.0.1",
			ListenPort:    53331,
			TargetHost:    "172.16.16.6",
			TargetPort:    53,
			Protocol:      forward.ProtocolBoth,
		},
		{
			ListenAddress: "127.0.0.1",
			ListenPort:    53332,
			TargetHost:    "10.50.10.1",
			TargetPort:    53,
			Protocol:      forward.ProtocolTCP,
		},
		{
			ListenAddress: "127.0.0.1",
			ListenPort:    53333,
			TargetHost:    "10.50.10.1",
			TargetPort:    53,
			Protocol:      forward.ProtocolBoth,
		},
	}

	matching := matchingForwards(forwards, addr, 53)
	if len(matching) != 1 {
		t.Fatalf("expected one matching forward, got %d", len(matching))
	}
	if matching[0].ListenPort != 53333 {
		t.Fatalf("expected port 53333, got %d", matching[0].ListenPort)
	}
}

func TestForwardInUseIgnoresRemovingDomains(t *testing.T) {
	state := state{Entries: map[string]stateEntry{
		"one.test": {ForwardListenPort: 53331},
		"two.test": {ForwardListenPort: 53331},
	}}

	if !forwardInUse(state, 53331, []string{"one.test"}) {
		t.Fatal("expected forward to remain in use by two.test")
	}
	if forwardInUse(state, 53331, []string{"one.test", "two.test"}) {
		t.Fatal("expected forward to be unused when all domains are removed")
	}
}

func TestShouldRemoveReplacedForwardOnlyForUnusedAutoForward(t *testing.T) {
	previous := stateEntry{ForwardListenPort: 53331, ForwardOwner: forwardOwnerDNSForward}
	state := state{Entries: map[string]stateEntry{
		"current.test": {ForwardListenPort: 53332, ForwardOwner: forwardOwnerDNSForward},
	}}

	if !shouldRemoveReplacedForward(previous, true, 53332, state) {
		t.Fatal("expected replaced auto-created forward to be removed")
	}

	state.Entries["other.test"] = stateEntry{ForwardListenPort: 53331, ForwardOwner: forwardOwnerDNSForward}
	if shouldRemoveReplacedForward(previous, true, 53332, state) {
		t.Fatal("expected shared forward to remain")
	}

	previous.ForwardOwner = ""
	if shouldRemoveReplacedForward(previous, true, 53332, state) {
		t.Fatal("expected pre-existing forward to remain")
	}
}

func TestShouldRemoveForwardOnDeleteRequiresDNSForwardOwnership(t *testing.T) {
	state := state{Entries: map[string]stateEntry{
		"example.test": {
			ForwardListenPort: 53331,
			ForwardOwner:      forwardOwnerDNSForward,
		},
	}}

	if !shouldRemoveForwardOnDelete(state.Entries["example.test"], true, state, []string{"example.test"}) {
		t.Fatal("expected dns-forward-owned forward to be removed when last domain is removed")
	}

	entry := state.Entries["example.test"]
	entry.ForwardOwner = ""
	if shouldRemoveForwardOnDelete(entry, true, state, []string{"example.test"}) {
		t.Fatal("expected existing forward without ownership to remain")
	}
}

func TestShouldRemoveForwardOnDeleteKeepsSharedForward(t *testing.T) {
	state := state{Entries: map[string]stateEntry{
		"one.test": {
			ForwardListenPort: 53331,
			ForwardOwner:      forwardOwnerDNSForward,
		},
		"two.test": {
			ForwardListenPort: 53331,
			ForwardOwner:      forwardOwnerDNSForward,
		},
	}}

	if shouldRemoveForwardOnDelete(state.Entries["one.test"], true, state, []string{"one.test"}) {
		t.Fatal("expected shared forward to remain while another dns-forward entry uses it")
	}
}
