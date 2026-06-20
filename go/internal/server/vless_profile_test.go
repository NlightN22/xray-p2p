package server

import (
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func TestBuildVLESSTLSVisionInbound(t *testing.T) {
	cfg := xrayconfig.ServerXrayConfig{}
	cfg.Inbounds.Trojan.Tag = "tunnel"
	cfg.Inbounds.Trojan.Listen = "0.0.0.0"
	inbound := buildTunnelInbound(cfg, tunnel.ProfileVLESSTLSVision, 443, "cert.pem", "key.pem", false, []trojanClient{{
		Email: "alice", Password: "550e8400-e29b-41d4-a716-446655440000",
	}})
	if inbound["protocol"] != "vless" {
		t.Fatalf("protocol = %v, want vless", inbound["protocol"])
	}
	settings := inbound["settings"].(map[string]any)
	clients := settings["clients"].([]any)
	user := clients[0].(map[string]any)
	if user["id"] != "550e8400-e29b-41d4-a716-446655440000" || user["flow"] != "xtls-rprx-vision" {
		t.Fatalf("unexpected VLESS user: %#v", user)
	}
}
