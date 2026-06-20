package client

import "testing"

func TestVLESSTLSVisionOutbound(t *testing.T) {
	outbound, err := tunnelOutbound(clientEndpointRecord{
		Profile: "vless-tls-vision", Protocol: "vless", Transport: "tcp", Security: "tls", Flow: "xtls-rprx-vision",
		Hostname: "edge.example", Address: "edge.example", Port: 443, User: "alice", Password: "550e8400-e29b-41d4-a716-446655440000", ServerName: "edge.example", Tag: "proxy-edge",
	}, nil, false)
	if err != nil {
		t.Fatalf("tunnelOutbound: %v", err)
	}
	entry := outbound.(map[string]any)
	if entry["protocol"] != "vless" {
		t.Fatalf("protocol = %v, want vless", entry["protocol"])
	}
	settings := entry["settings"].(map[string]any)
	server := settings["vnext"].([]any)[0].(map[string]any)
	user := server["users"].([]any)[0].(map[string]any)
	if user["flow"] != "xtls-rprx-vision" || user["encryption"] != "none" {
		t.Fatalf("unexpected VLESS user: %#v", user)
	}
}
