package server

import (
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func TestBuildVLESSTLSVisionInbound(t *testing.T) {
	cfg := xrayconfig.ServerXrayConfig{}
	cfg.Inbounds.Trojan.Tag = "tunnel"
	cfg.Inbounds.Trojan.Listen = "0.0.0.0"
	inbound := buildTunnelInbound(cfg, tunnel.ProfileVLESSTLSVision, 443, "cert.pem", "key.pem", false, []trojanClient{
		{Email: "alice", Password: "550e8400-e29b-41d4-a716-446655440000", PreviousCredentialForRotation: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", RotationExpiresAt: time.Now().Add(time.Hour)},
		{Email: "legacy", Password: "6ba7b811-9dad-11d1-80b4-00c04fd430c8", PreviousCredentialForRotation: "not-a-uuid", RotationExpiresAt: time.Now().Add(time.Hour)},
		{Email: "disabled", Password: "6ba7b812-9dad-11d1-80b4-00c04fd430c8", PreviousCredentialForRotation: "6ba7b813-9dad-11d1-80b4-00c04fd430c8", RotationExpiresAt: time.Now().Add(time.Hour), Disabled: true},
	})
	if inbound["protocol"] != "vless" {
		t.Fatalf("protocol = %v, want vless", inbound["protocol"])
	}
	settings := inbound["settings"].(map[string]any)
	clients := settings["clients"].([]any)
	user := clients[0].(map[string]any)
	if user["id"] != "550e8400-e29b-41d4-a716-446655440000" || user["flow"] != "xtls-rprx-vision" {
		t.Fatalf("unexpected VLESS user: %#v", user)
	}
	if len(clients) != 3 {
		t.Fatalf("VLESS clients = %d, want active, previous, and legacy active", len(clients))
	}
	previous := clients[1].(map[string]any)
	if previous["id"] != "6ba7b810-9dad-11d1-80b4-00c04fd430c8" || previous["email"] != "alice.previous" {
		t.Fatalf("unexpected previous VLESS user: %#v", previous)
	}
}
