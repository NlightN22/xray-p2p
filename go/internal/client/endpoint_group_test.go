package client

import (
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func TestSelectEndpointGroupThresholdCooldownAndManualMode(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	group := endpointGroup{GroupID: "g1", Tag: "group-main", Members: []string{"primary", "backup"}, CooldownSeconds: 10}
	endpoints := []clientEndpointRecord{{Tag: "primary"}, {Tag: "backup"}}
	if got, ok := selectEndpointGroup(group, endpoints, endpointGroupSelector{ActiveTag: "primary", Failures: map[string]int{"primary": 1}}, now); !ok || got != "primary" {
		t.Fatalf("one failure selected %q, %v", got, ok)
	}
	if got, ok := selectEndpointGroup(group, endpoints, endpointGroupSelector{ActiveTag: "primary", Failures: map[string]int{"primary": 2}}, now); !ok || got != "backup" {
		t.Fatalf("threshold selected %q, %v", got, ok)
	}
	group.Mode, group.ManualActiveTag = endpointGroupModeManual, "backup"
	if got, ok := selectEndpointGroup(group, endpoints, endpointGroupSelector{}, now); !ok || got != "backup" {
		t.Fatalf("manual selected %q, %v", got, ok)
	}
}

func TestBuildClientOutboundsResolvesLogicalGroupAndFailsClosed(t *testing.T) {
	desired := clientInstallState{
		Endpoints:      []clientEndpointRecord{{Tag: "primary", Address: "198.51.100.1", Port: 443, Password: "secret"}},
		EndpointGroups: []endpointGroup{{GroupID: "g1", Tag: "group-main", Members: []string{"primary"}}},
	}
	outbounds, err := buildClientOutboundsWithSelector(xrayconfig.DefaultClientConfig().DirectOutbound, desired, nil, false, endpointSelectorState{})
	if err != nil {
		t.Fatal(err)
	}
	if len(outbounds) < 2 {
		t.Fatalf("outbounds = %#v", outbounds)
	}
	group, _ := outbounds[1].(struct {
		Protocol       string         `json:"protocol"`
		Settings       trojanSettings `json:"settings"`
		StreamSettings streamSettings `json:"streamSettings"`
		Tag            string         `json:"tag"`
	})
	if group.Tag != "group-main" {
		t.Fatalf("logical group tag = %q", group.Tag)
	}
	desired.Endpoints[0].Disabled = true
	outbounds, err = buildClientOutboundsWithSelector(xrayconfig.DefaultClientConfig().DirectOutbound, desired, nil, false, endpointSelectorState{})
	if err != nil {
		t.Fatal(err)
	}
	blackhole, _ := outbounds[0].(map[string]any)
	if blackhole["protocol"] != "blackhole" || blackhole["tag"] != "group-main" {
		t.Fatalf("expected fail-closed group outbound, got %#v", outbounds[0])
	}
}

func TestSelectEndpointGroupFailsClosed(t *testing.T) {
	group := endpointGroup{GroupID: "g1", Tag: "group-main", Members: []string{"primary"}}
	if _, ok := selectEndpointGroup(group, []clientEndpointRecord{{Tag: "primary", Disabled: true}}, endpointGroupSelector{}, time.Now()); ok {
		t.Fatal("disabled-only group must fail closed")
	}
}
