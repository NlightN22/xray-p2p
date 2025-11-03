package cli

import (
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestParsePingArgsSocksFromConfig(t *testing.T) {
	cfg := config.Config{}
	cfg.Client.SocksAddress = "127.0.0.1:1080"

	host, opts, err := parsePingArgs(cfg, []string{"example.com", "--socks"})
	if err != nil {
		t.Fatalf("parsePingArgs returned error: %v", err)
	}
	if host != "example.com" {
		t.Fatalf("expected host example.com, got %s", host)
	}
	if opts.SocksProxy != "127.0.0.1:1080" {
		t.Fatalf("expected socks proxy 127.0.0.1:1080, got %s", opts.SocksProxy)
	}
}

func TestParsePingArgsSocksExplicit(t *testing.T) {
	cfg := config.Config{}

	host, opts, err := parsePingArgs(cfg, []string{"--socks", "10.0.0.10:9000", "target.local"})
	if err != nil {
		t.Fatalf("parsePingArgs returned error: %v", err)
	}
	if host != "target.local" {
		t.Fatalf("expected host target.local, got %s", host)
	}
	if opts.SocksProxy != "10.0.0.10:9000" {
		t.Fatalf("expected socks proxy 10.0.0.10:9000, got %s", opts.SocksProxy)
	}
}

func TestParsePingArgsSocksMissingConfig(t *testing.T) {
	cfg := config.Config{}

	_, _, err := parsePingArgs(cfg, []string{"example.org", "--socks"})
	if err == nil {
		t.Fatal("expected error when socks address missing in config")
	}
	if got := err.Error(); !strings.Contains(got, "SOCKS proxy address is not configured") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParsePingArgsInvalidSocksAddress(t *testing.T) {
	cfg := config.Config{}

	_, _, err := parsePingArgs(cfg, []string{"example.org", "--socks", "bad-port"})
	if err == nil {
		t.Fatal("expected error for invalid socks address")
	}
	if got := err.Error(); !strings.Contains(got, "expected host:port") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
