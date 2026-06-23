package server

import (
	"encoding/json"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestMergeHAOwnedRedirectsPreservesUserRules(t *testing.T) {
	user := redirect.Rule{CIDR: "10.1.0.0/16", OutboundTag: "user-channel"}
	oldHA := redirect.Rule{Domain: "old.example", OutboundTag: "ha-channel"}
	newHA := redirect.Rule{Domain: "new.example", OutboundTag: "ha-channel"}
	payload, err := json.Marshal([]redirect.Rule{newHA})
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		serverRedirectRulesKey:  []redirect.Rule{user, oldHA},
		serverHARedirectKeysKey: []string{haRedirectKey(oldHA)},
	}
	merged, err := mergeHAOwnedRedirects(doc, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 2 || merged[0].OutboundTag != user.OutboundTag || merged[0].CIDR != user.CIDR || merged[1].OutboundTag != newHA.OutboundTag || merged[1].Domain != newHA.Domain {
		t.Fatalf("merged redirects = %+v", merged)
	}
}

func TestMergeHAOwnedRedirectsClearsOnlyHAEntries(t *testing.T) {
	user := redirect.Rule{CIDR: "10.1.0.0/16", OutboundTag: "user-channel"}
	haRule := redirect.Rule{Domain: "old.example", OutboundTag: "ha-channel"}
	doc := map[string]any{
		serverRedirectRulesKey:  []redirect.Rule{user, haRule},
		serverHARedirectKeysKey: []string{haRedirectKey(haRule)},
	}
	merged, err := mergeHAOwnedRedirects(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 1 || merged[0].OutboundTag != user.OutboundTag || merged[0].CIDR != user.CIDR {
		t.Fatalf("merged redirects = %+v", merged)
	}
}
