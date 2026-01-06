package clientcmd

import (
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"strconv"
)

func TestNormalizeServerPortPrefersFlag(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{Port: "8443"},
		Client: config.ClientConfig{ServerPort: "62022"},
	}
	got := normalizeServerPort(cfg, "12345")
	if got != "12345" {
		t.Fatalf("expected flag port to win, got %q", got)
	}
}

func TestNormalizeServerPortPrefersClientConfig(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{Port: "8443"},
		Client: config.ClientConfig{ServerPort: "62022"},
	}
	got := normalizeServerPort(cfg, "")
	if got != "62022" {
		t.Fatalf("expected client server_port, got %q", got)
	}
}

func TestNormalizeServerPortFallsBackToServerConfig(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{Port: "62022"},
		Client: config.ClientConfig{ServerPort: ""},
	}
	got := normalizeServerPort(cfg, "")
	if got != "62022" {
		t.Fatalf("expected server port, got %q", got)
	}
}

func TestNormalizeServerPortUsesDefaultWhenEmpty(t *testing.T) {
	cfg := config.Config{}
	got := normalizeServerPort(cfg, "")
	expected := server.DefaultTrojanPort
	want := strconv.Itoa(expected)
	if got != want {
		t.Fatalf("expected default trojan port %d, got %q", expected, got)
	}
}
