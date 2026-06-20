package clientcmd

import "testing"

func TestParseInstallLinkVLESS(t *testing.T) {
	link, err := parseInstallLink("vless://550e8400-e29b-41d4-a716-446655440000@edge.example.test:443?security=tls&type=tcp&sni=edge.example.test&flow=xtls-rprx-vision&encryption=none#alice")
	if err != nil {
		t.Fatalf("parse VLESS install link: %v", err)
	}
	if link.Profile != "vless-tls-vision" || link.Protocol != "vless" || link.Flow != "xtls-rprx-vision" {
		t.Fatalf("unexpected VLESS link: %+v", link)
	}
}
