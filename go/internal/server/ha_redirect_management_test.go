package server

import (
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/ha"
)

func TestHAGroupRedirectLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.toml")
	generation := ha.Generation{
		Number: 1,
		Group: ha.Group{
			ID:       "group-1",
			Tag:      "ha-group",
			Selector: ha.Selector{Mode: "automatic"},
			Members:  []ha.Member{{ID: "member-1", Tag: "endpoint-1", Host: "server.example", Port: 443, Profile: "trojan-tls", Confirmed: true}},
		},
		Channels: []ha.Channel{{ID: "portal", Tag: "ha-portal", Domain: "portal.example", Binding: ha.ChannelBinding{GroupTag: "ha-group"}}},
	}
	if err := writeServerStateDoc(path, map[string]any{serverHAGenerationKey: generation}); err != nil {
		t.Fatal(err)
	}
	added, err := AddHARedirect(path, "portal", "10.70.0.0/16", "")
	if err != nil {
		t.Fatal(err)
	}
	if added.Number != 2 {
		t.Fatalf("generation = %d", added.Number)
	}
	rules, err := ListHARedirects(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].CIDR != "10.70.0.0/16" || rules[0].OutboundTag != "ha-portal" {
		t.Fatalf("rules = %+v", rules)
	}
	removed, err := RemoveHARedirect(path, "portal", "10.70.0.0/16", "")
	if err != nil {
		t.Fatal(err)
	}
	if removed.Number != 3 {
		t.Fatalf("generation = %d", removed.Number)
	}
	rules, err = ListHARedirects(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Fatalf("rules = %+v", rules)
	}
}

func TestHAGroupRedirectRejectsEndpointBoundChannel(t *testing.T) {
	generation := ha.Generation{
		Group:    ha.Group{ID: "group-1", Tag: "ha-group"},
		Channels: []ha.Channel{{ID: "endpoint", Tag: "endpoint-portal", Domain: "portal.example", Binding: ha.ChannelBinding{EndpointTag: "endpoint-1"}}},
	}
	if _, err := groupRedirectChannel(generation, "endpoint"); err == nil {
		t.Fatal("expected endpoint-bound channel rejection")
	}
}
