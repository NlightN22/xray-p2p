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

func TestRunServerRedirectRemove_TagOnlyDoesNotRequireCIDR(t *testing.T) {
	var captured server.RedirectRemoveOptions
	t.Cleanup(stubServerRedirectRemove(func(opts server.RedirectRemoveOptions) error {
		captured = opts
		return nil
	}))

	code := runServerRedirectRemove(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectRemoveOptions{
		Tag: "orphaned-host-example.rev",
	})
	if code != 0 {
		t.Fatalf("runServerRedirectRemove exit = %d, want 0", code)
	}
	if captured.Tag != "orphaned-host-example.rev" {
		t.Fatalf("unexpected tag: %+v", captured)
	}
	if captured.CIDR != "" || captured.Domain != "" {
		t.Fatalf("tag-only remove should not require target: %+v", captured)
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
