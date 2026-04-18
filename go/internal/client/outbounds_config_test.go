package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func TestWriteOutboundsConfigIncludesEndpointsAndFreedom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbounds.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example", Port: 8443, User: "alpha", Password: "secret", ServerName: "alpha.example"},
		{Hostname: "beta.example", Tag: "proxy-beta", Address: "beta.example", Port: 9443, User: "beta", Password: "other", ServerName: "beta.example"},
	}

	if err := writeOutboundsConfig(path, xrayconfig.DefaultClientConfig().DirectOutbound, endpoints, nil, false); err != nil {
		t.Fatalf("writeOutboundsConfig failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read outbounds: %v", err)
	}
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse outbounds: %v", err)
	}
	if len(doc.Outbounds) != 3 {
		t.Fatalf("expected 3 outbounds, got %d", len(doc.Outbounds))
	}
	if doc.Outbounds[0]["tag"] != "proxy-alpha" || doc.Outbounds[1]["tag"] != "proxy-beta" {
		t.Fatalf("unexpected tags: %+v", doc.Outbounds)
	}
	if doc.Outbounds[2]["tag"] != "direct" {
		t.Fatalf("expected last outbound to be direct, got %+v", doc.Outbounds[2])
	}
}

func TestWriteOutboundsConfigPreservesUserOutbounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbounds.json")
	directTag := "direct"
	existing := map[string]any{
		"outbounds": []any{
			map[string]any{
				"tag":      "user-proxy",
				"protocol": "socks",
			},
			map[string]any{
				"tag":         directTag,
				"protocol":    "freedom",
				"sendThrough": "192.0.2.50",
			},
		},
	}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal existing: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example", Port: 8443, User: "alpha", Password: "secret", ServerName: "alpha.example"},
	}
	cfg := xrayconfig.DefaultClientConfig().DirectOutbound
	cfg.SendThrough = "10.0.0.5"
	if err := writeOutboundsConfig(path, cfg, endpoints, nil, false); err != nil {
		t.Fatalf("writeOutboundsConfig failed: %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read outbounds: %v", err)
	}
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(updated, &doc); err != nil {
		t.Fatalf("parse outbounds: %v", err)
	}

	var hasUser, hasDirect, hasProxy bool
	var directSendThrough string
	for _, outbound := range doc.Outbounds {
		tag, _ := outbound["tag"].(string)
		switch tag {
		case "user-proxy":
			hasUser = true
		case directTag:
			hasDirect = true
			directSendThrough, _ = outbound["sendThrough"].(string)
		case "proxy-alpha":
			hasProxy = true
		}
	}
	if !hasUser || !hasProxy || !hasDirect {
		t.Fatalf("expected user, proxy, and direct outbounds, got %+v", doc.Outbounds)
	}
	if directSendThrough != "10.0.0.5" {
		t.Fatalf("expected sendThrough to be updated, got %q", directSendThrough)
	}
	if tag, _ := doc.Outbounds[0]["tag"].(string); tag != "user-proxy" {
		t.Fatalf("expected user outbound to stay first, got %+v", doc.Outbounds[0])
	}
}
