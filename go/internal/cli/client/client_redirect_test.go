package clientcmd

import (
	"context"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
)

func TestRunClientRedirectAdd_PromptSelection(t *testing.T) {
	t.Cleanup(stubClientRedirectAdd(func(opts client.RedirectAddOptions) error {
		if opts.Tag != "proxy-b" {
			t.Fatalf("Tag mismatch: got %s want proxy-b", opts.Tag)
		}
		if opts.Hostname != "edge-b" {
			t.Fatalf("Host mismatch: got %s want edge-b", opts.Hostname)
		}
		return nil
	}))
	t.Cleanup(stubClientList(func(client.ListOptions) ([]client.EndpointRecord, error) {
		return []client.EndpointRecord{
			{Tag: "proxy-a", Hostname: "edge-a"},
			{Tag: "proxy-b", Hostname: "edge-b"},
		}, nil
	}))
	t.Cleanup(stubClientRedirectPromptReader(strings.NewReader("2\n")))

	code := runClientRedirectAdd(context.Background(), clientCfg("C:\\xp2p", "cfg"), []string{"--cidr", "10.0.0.0/24"})
	if code != 0 {
		t.Fatalf("runClientRedirectAdd exit = %d, want 0", code)
	}
}

func TestRunClientRedirectAdd_PromptCancelled(t *testing.T) {
	called := false
	t.Cleanup(stubClientRedirectAdd(func(client.RedirectAddOptions) error {
		called = true
		return nil
	}))
	t.Cleanup(stubClientList(func(client.ListOptions) ([]client.EndpointRecord, error) {
		return []client.EndpointRecord{
			{Tag: "proxy-a", Hostname: "edge-a"},
		}, nil
	}))
	t.Cleanup(stubClientRedirectPromptReader(strings.NewReader("\n")))

	code := runClientRedirectAdd(context.Background(), clientCfg("C:\\xp2p", "cfg"), []string{"--cidr", "10.0.0.0/24"})
	if code != 2 {
		t.Fatalf("runClientRedirectAdd exit = %d, want 2", code)
	}
	if called {
		t.Fatalf("clientRedirectAddFunc called on cancelled prompt")
	}
}

func TestRunClientRedirectAdd_NoEndpoints(t *testing.T) {
	called := false
	t.Cleanup(stubClientRedirectAdd(func(client.RedirectAddOptions) error {
		called = true
		return nil
	}))
	t.Cleanup(stubClientList(func(client.ListOptions) ([]client.EndpointRecord, error) {
		return []client.EndpointRecord{}, nil
	}))
	t.Cleanup(stubClientRedirectPromptReader(strings.NewReader("1\n")))

	code := runClientRedirectAdd(context.Background(), clientCfg("C:\\xp2p", "cfg"), []string{"--cidr", "10.0.0.0/24"})
	if code != 2 {
		t.Fatalf("runClientRedirectAdd exit = %d, want 2", code)
	}
	if called {
		t.Fatalf("clientRedirectAddFunc called when no endpoints are available")
	}
}
