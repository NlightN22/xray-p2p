package server

import (
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func TestBuildVLESSLink(t *testing.T) {
	link, err := buildVLESSLink(
		"edge.example.test",
		443,
		"550e8400-e29b-41d4-a716-446655440000",
		"alice",
		tunnel.TLSMetadata{PinnedPeerCertSHA256: "pin", VerifyPeerCertByName: "edge.example.test"},
	)
	if err != nil {
		t.Fatalf("build VLESS link: %v", err)
	}
	for _, fragment := range []string{"vless://", "flow=xtls-rprx-vision", "encryption=none", "xp2p_pin_sha256=pin"} {
		if !strings.Contains(link, fragment) {
			t.Fatalf("VLESS link %q is missing %q", link, fragment)
		}
	}
}
