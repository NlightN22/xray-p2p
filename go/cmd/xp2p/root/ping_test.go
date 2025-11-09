package root

import "testing"

func TestResolveSocksAddressFromConfig(t *testing.T) {
	addr, err := resolveSocksAddress("127.0.0.1:1080", socksConfigSentinel)
	if err != nil {
		t.Fatalf("resolveSocksAddress returned error: %v", err)
	}
	if addr != "127.0.0.1:1080" {
		t.Fatalf("unexpected socks address: %s", addr)
	}
}

func TestResolveSocksAddressExplicit(t *testing.T) {
	addr, err := resolveSocksAddress("", "10.0.0.10:9000")
	if err != nil {
		t.Fatalf("resolveSocksAddress returned error: %v", err)
	}
	if addr != "10.0.0.10:9000" {
		t.Fatalf("unexpected socks address: %s", addr)
	}
}

func TestResolveSocksAddressMissingConfig(t *testing.T) {
	if _, err := resolveSocksAddress("", socksConfigSentinel); err == nil {
		t.Fatal("expected error when config socks address is empty")
	}
}

func TestResolveSocksAddressInvalid(t *testing.T) {
	if _, err := resolveSocksAddress("", "bad-port"); err == nil {
		t.Fatal("expected error for invalid socks address")
	}
}
