package clientcmd

import (
	"context"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
)

func TestRunClientRedirectAdd_QuietAmbiguousBinding(t *testing.T) {
	called := false
	t.Cleanup(stubClientRedirectAdd(func(client.RedirectAddOptions) error {
		called = true
		return nil
	}))
	t.Cleanup(stubClientList(func(client.ListOptions) ([]client.EndpointRecord, error) {
		return []client.EndpointRecord{
			{Tag: "proxy-a", Hostname: "edge-a"},
			{Tag: "proxy-b", Hostname: "edge-b"},
		}, nil
	}))

	code := runClientRedirectAdd(context.Background(), clientCfg("C:\\xp2p", "cfg"), []string{
		"--cidr", "10.0.0.0/24",
		"--quiet",
	})
	if code != 2 {
		t.Fatalf("runClientRedirectAdd exit = %d, want 2", code)
	}
	if called {
		t.Fatalf("clientRedirectAddFunc called for ambiguous quiet binding")
	}
}

func TestRunClientRedirectRemove_QuietAmbiguousBinding(t *testing.T) {
	called := false
	t.Cleanup(stubClientRedirectRemove(func(client.RedirectRemoveOptions) error {
		called = true
		return nil
	}))
	t.Cleanup(stubClientRedirectList(func(client.RedirectListOptions) ([]client.RedirectRecord, error) {
		return []client.RedirectRecord{
			{Type: "CIDR", Value: "10.0.0.0/24", CIDR: "10.0.0.0/24", Tag: "proxy-a", Hostname: "edge-a"},
			{Type: "CIDR", Value: "10.0.0.0/24", CIDR: "10.0.0.0/24", Tag: "proxy-b", Hostname: "edge-b"},
		}, nil
	}))

	code := runClientRedirectRemove(context.Background(), clientCfg("C:\\xp2p", "cfg"), []string{
		"--cidr", "10.0.0.0/24",
		"--quiet",
	})
	if code != 2 {
		t.Fatalf("runClientRedirectRemove exit = %d, want 2", code)
	}
	if called {
		t.Fatalf("clientRedirectRemoveFunc called for ambiguous quiet binding")
	}
}

func TestRunClientRedirectToggle_QuietAmbiguousBinding(t *testing.T) {
	called := false
	t.Cleanup(stubClientRedirectToggle(func(client.RedirectSetEnabledOptions) error {
		called = true
		return nil
	}))
	t.Cleanup(stubClientRedirectList(func(client.RedirectListOptions) ([]client.RedirectRecord, error) {
		return []client.RedirectRecord{
			{Type: "CIDR", Value: "10.0.0.0/24", CIDR: "10.0.0.0/24", Tag: "proxy-a", Hostname: "edge-a"},
			{Type: "CIDR", Value: "10.0.0.0/24", CIDR: "10.0.0.0/24", Tag: "proxy-b", Hostname: "edge-b"},
		}, nil
	}))

	code := runClientRedirectToggle(context.Background(), clientCfg("C:\\xp2p", "cfg"), []string{
		"--cidr", "10.0.0.0/24",
		"--quiet",
	}, false)
	if code != 2 {
		t.Fatalf("runClientRedirectToggle exit = %d, want 2", code)
	}
	if called {
		t.Fatalf("clientRedirectToggleFunc called for ambiguous quiet binding")
	}
}
