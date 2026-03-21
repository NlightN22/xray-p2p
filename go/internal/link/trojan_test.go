package link

import "testing"

func TestParseTrojanLink(t *testing.T) {
	link, err := ParseTrojanLink("trojan://secret@edge.example.com:8443?security=tls&sni=edge.example.com#user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.ServerAddress != "edge.example.com" {
		t.Fatalf("unexpected server address: %s", link.ServerAddress)
	}
	if link.ServerPort != "8443" {
		t.Fatalf("unexpected server port: %s", link.ServerPort)
	}
	if link.User != "user@example.com" {
		t.Fatalf("unexpected user: %s", link.User)
	}
	if link.Password != "secret" {
		t.Fatalf("unexpected password: %s", link.Password)
	}
	if link.ServerName != "edge.example.com" {
		t.Fatalf("unexpected server name: %s", link.ServerName)
	}
	if !link.ServerNameSet {
		t.Fatal("expected server name to be set")
	}
	if link.ALPN != nil {
		t.Fatalf("unexpected alpn: %v", link.ALPN)
	}
}

func TestParseTrojanLinkAllowsInsecure(t *testing.T) {
	link, err := ParseTrojanLink("trojan://secret@edge.example.com:443?allowInsecure=1#user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !link.AllowInsecure {
		t.Fatal("expected allow insecure")
	}
	if !link.AllowInsecureSet {
		t.Fatal("expected allow insecure flag set")
	}
}

func TestParseTrojanLinkALPN(t *testing.T) {
	link, err := ParseTrojanLink("trojan://secret@edge.example.com:443?alpn=h2,http/1.1&security=tls&sni=edge.example.com#user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(link.ALPN) != 2 {
		t.Fatalf("unexpected alpn count: %v", link.ALPN)
	}
	if link.ALPN[0] != "h2" || link.ALPN[1] != "http/1.1" {
		t.Fatalf("unexpected alpn values: %v", link.ALPN)
	}
}

func TestParseTrojanLinkRejectsInvalid(t *testing.T) {
	if _, err := ParseTrojanLink(""); err == nil {
		t.Fatal("expected error for empty link")
	}
	if _, err := ParseTrojanLink("http://example.com"); err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}
