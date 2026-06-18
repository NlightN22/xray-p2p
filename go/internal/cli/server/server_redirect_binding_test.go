package servercmd

import (
	"context"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerRedirectAdd_QuietAmbiguousBinding(t *testing.T) {
	called := false
	t.Cleanup(stubServerRedirectAdd(func(server.RedirectAddOptions) error {
		called = true
		return nil
	}))
	t.Cleanup(stubServerReverseList(func(server.ReverseListOptions) ([]server.ReverseRecord, error) {
		return []server.ReverseRecord{
			{Tag: "alpha.rev", Host: "edge-a"},
			{Tag: "beta.rev", Host: "edge-b"},
		}, nil
	}))

	code := runServerRedirectAdd(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectAddOptions{
		CIDR:  "10.10.0.0/16",
		Quiet: true,
	})
	if code != 2 {
		t.Fatalf("runServerRedirectAdd exit = %d, want 2", code)
	}
	if called {
		t.Fatalf("serverRedirectAddFunc called for ambiguous quiet binding")
	}
}

func TestRunServerRedirectRemove_QuietAmbiguousBinding(t *testing.T) {
	called := false
	t.Cleanup(stubServerRedirectRemove(func(server.RedirectRemoveOptions) error {
		called = true
		return nil
	}))
	t.Cleanup(stubServerRedirectList(func(server.RedirectListOptions) ([]server.RedirectRecord, error) {
		return []server.RedirectRecord{
			{Type: "CIDR", Value: "10.10.0.0/16", CIDR: "10.10.0.0/16", Tag: "alpha.rev", Hostname: "edge-a"},
			{Type: "CIDR", Value: "10.10.0.0/16", CIDR: "10.10.0.0/16", Tag: "beta.rev", Hostname: "edge-b"},
		}, nil
	}))

	code := runServerRedirectRemove(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectRemoveOptions{
		CIDR:  "10.10.0.0/16",
		Quiet: true,
	})
	if code != 2 {
		t.Fatalf("runServerRedirectRemove exit = %d, want 2", code)
	}
	if called {
		t.Fatalf("serverRedirectRemoveFunc called for ambiguous quiet binding")
	}
}

func TestRunServerRedirectToggle_QuietAmbiguousBinding(t *testing.T) {
	called := false
	t.Cleanup(stubServerRedirectToggle(func(server.RedirectSetEnabledOptions) error {
		called = true
		return nil
	}))
	t.Cleanup(stubServerRedirectList(func(server.RedirectListOptions) ([]server.RedirectRecord, error) {
		return []server.RedirectRecord{
			{Type: "CIDR", Value: "10.10.0.0/16", CIDR: "10.10.0.0/16", Tag: "alpha.rev", Hostname: "edge-a"},
			{Type: "CIDR", Value: "10.10.0.0/16", CIDR: "10.10.0.0/16", Tag: "beta.rev", Hostname: "edge-b"},
		}, nil
	}))

	code := runServerRedirectToggle(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectToggleOptions{
		CIDR:  "10.10.0.0/16",
		Quiet: true,
	}, false)
	if code != 2 {
		t.Fatalf("runServerRedirectToggle exit = %d, want 2", code)
	}
	if called {
		t.Fatalf("serverRedirectToggleFunc called for ambiguous quiet binding")
	}
}
