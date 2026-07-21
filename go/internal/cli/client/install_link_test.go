package clientcmd

import (
	"strings"
	"testing"
)

func TestParseInstallLinkVLESS(t *testing.T) {
	link, err := parseInstallLink("vless://550e8400-e29b-41d4-a716-446655440000@edge.example.test:443?security=tls&type=tcp&sni=edge.example.test&flow=xtls-rprx-vision&encryption=none#alice")
	if err != nil {
		t.Fatalf("parse VLESS install link: %v", err)
	}
	if link.Profile != "vless-tls-vision" || link.Protocol != "vless" || link.Flow != "xtls-rprx-vision" {
		t.Fatalf("unexpected VLESS link: %+v", link)
	}
}

func TestParseInstallLinkRejectsUnknownRequiredParameter(t *testing.T) {
	const secret = "do-not-disclose-this-password"
	_, err := parseInstallLink("trojan://" + secret + "@edge.example.test:443?security=tls&type=tcp&requiredFeature=unsupported")
	if err == nil {
		t.Fatal("expected unsupported parameter error")
	}
	if !strings.Contains(err.Error(), "requiredFeature") {
		t.Fatalf("error does not identify unsupported parameter: %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error discloses connection credential: %v", err)
	}
}

func TestParseInstallLinkAllowsUnknownOptionalParameter(t *testing.T) {
	_, err := parseInstallLink("trojan://secret@edge.example.test:443?security=tls&type=tcp&x-optional-client-note=ignored")
	if err != nil {
		t.Fatalf("parse link with optional extension: %v", err)
	}
}
