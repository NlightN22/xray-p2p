package servercmd

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerRedirectAddValidatesInputs(t *testing.T) {
	t.Cleanup(func() {
		serverRedirectAddFunc = server.AddRedirect
	})

	var captured server.RedirectAddOptions
	serverRedirectAddFunc = func(opts server.RedirectAddOptions) error {
		captured = opts
		return nil
	}

	cfg := config.Config{}
	cfg.Server.InstallDir = "C:\\srv"
	cfg.Server.ConfigDir = "config-server"
	code := runServerRedirectAdd(context.Background(), cfg, serverRedirectAddOptions{
		CIDR: "10.70.0.0/16",
		Tag:  "alphaedge-example.rev",
	})
	if code != 0 {
		t.Fatalf("runServerRedirectAdd returned %d", code)
	}
	want := server.RedirectAddOptions{
		InstallDir: "C:\\srv",
		ConfigDir:  "config-server",
		CIDR:       "10.70.0.0/16",
		Tag:        "alphaedge-example.rev",
	}
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("captured add options %+v, want %+v", captured, want)
	}

	code = runServerRedirectAdd(context.Background(), cfg, serverRedirectAddOptions{})
	if code != 2 {
		t.Fatalf("expected validation error, got %d", code)
	}
}

func TestRunServerRedirectRemoveHandlesErrors(t *testing.T) {
	t.Cleanup(func() {
		serverRedirectRemoveFunc = server.RemoveRedirect
	})
	serverRedirectRemoveFunc = func(server.RedirectRemoveOptions) error {
		return errors.New("boom")
	}
	cfg := config.Config{}
	code := runServerRedirectRemove(context.Background(), cfg, serverRedirectRemoveOptions{
		CIDR: "10.60.0.0/16",
		Tag:  "alpha.rev",
	})
	if code != 1 {
		t.Fatalf("expected failure exit code, got %d", code)
	}
}

func TestRunServerRedirectListPrintsEmpty(t *testing.T) {
	t.Cleanup(func() {
		serverRedirectListFunc = server.ListRedirects
	})
	serverRedirectListFunc = func(server.RedirectListOptions) ([]server.RedirectRecord, error) {
		return nil, nil
	}
	cfg := config.Config{}
	code := runServerRedirectList(context.Background(), cfg, serverRedirectListOptions{})
	if code != 0 {
		t.Fatalf("expected success, got %d", code)
	}
}

func TestRunServerRedirectAdd_PromptSelection(t *testing.T) {
	t.Cleanup(stubServerRedirectAdd(func(opts server.RedirectAddOptions) error {
		if opts.Tag != "alpha.rev" {
			t.Fatalf("Tag mismatch: got %s want alpha.rev", opts.Tag)
		}
		if opts.Hostname != "edge-a" {
			t.Fatalf("Host mismatch: got %s want edge-a", opts.Hostname)
		}
		return nil
	}))
	t.Cleanup(stubServerReverseList(func(server.ReverseListOptions) ([]server.ReverseRecord, error) {
		return []server.ReverseRecord{
			{Tag: "alpha.rev", Host: "edge-a"},
			{Tag: "beta.rev", Host: "edge-b"},
		}, nil
	}))
	t.Cleanup(stubServerRedirectPromptReader(strings.NewReader("1\n")))

	code := runServerRedirectAdd(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectAddOptions{
		CIDR: "10.10.0.0/16",
	})
	if code != 0 {
		t.Fatalf("runServerRedirectAdd exit = %d, want 0", code)
	}
}

func TestRunServerRedirectAdd_PromptCancelled(t *testing.T) {
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
	t.Cleanup(stubServerRedirectPromptReader(strings.NewReader("\n")))

	code := runServerRedirectAdd(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectAddOptions{
		CIDR: "10.10.0.0/16",
	})
	if code != 2 {
		t.Fatalf("runServerRedirectAdd exit = %d, want 2", code)
	}
	if called {
		t.Fatalf("serverRedirectAddFunc called on cancelled prompt")
	}
}

func TestRunServerRedirectAdd_NoReverseChannels(t *testing.T) {
	called := false
	t.Cleanup(stubServerRedirectAdd(func(server.RedirectAddOptions) error {
		called = true
		return nil
	}))
	t.Cleanup(stubServerReverseList(func(server.ReverseListOptions) ([]server.ReverseRecord, error) {
		return []server.ReverseRecord{}, nil
	}))
	t.Cleanup(stubServerRedirectPromptReader(strings.NewReader("1\n")))

	code := runServerRedirectAdd(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectAddOptions{
		CIDR: "10.10.0.0/16",
	})
	if code != 2 {
		t.Fatalf("runServerRedirectAdd exit = %d, want 2", code)
	}
	if called {
		t.Fatalf("serverRedirectAddFunc called when no reverse channels are available")
	}
}

func TestRunServerRedirectAdd_SinglePortalSkipsPrompt(t *testing.T) {
	t.Cleanup(stubServerRedirectAdd(func(opts server.RedirectAddOptions) error {
		if opts.Tag != "alpha.rev" || opts.Hostname != "edge-a" {
			t.Fatalf("binding mismatch: got %s/%s want alpha.rev/edge-a", opts.Tag, opts.Hostname)
		}
		return nil
	}))
	t.Cleanup(stubServerReverseList(func(server.ReverseListOptions) ([]server.ReverseRecord, error) {
		return []server.ReverseRecord{{Tag: "alpha.rev", Host: "edge-a"}}, nil
	}))
	t.Cleanup(stubServerRedirectPromptReader(strings.NewReader("")))

	code := runServerRedirectAdd(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectAddOptions{
		CIDR: "10.10.0.0/16",
	})
	if code != 0 {
		t.Fatalf("runServerRedirectAdd exit = %d, want 0", code)
	}
}

func TestRunServerRedirectRemove_SingleMatchingRedirectSkipsPrompt(t *testing.T) {
	t.Cleanup(stubServerRedirectRemove(func(opts server.RedirectRemoveOptions) error {
		if opts.Tag != "alpha.rev" || opts.Hostname != "edge-a" {
			t.Fatalf("binding mismatch: got %s/%s want alpha.rev/edge-a", opts.Tag, opts.Hostname)
		}
		return nil
	}))
	t.Cleanup(stubServerRedirectList(func(server.RedirectListOptions) ([]server.RedirectRecord, error) {
		return []server.RedirectRecord{
			{Type: "CIDR", Value: "10.10.0.0/16", CIDR: "10.10.0.0/16", Tag: "alpha.rev", Hostname: "edge-a"},
			{Type: "CIDR", Value: "10.20.0.0/16", CIDR: "10.20.0.0/16", Tag: "beta.rev", Hostname: "edge-b"},
		}, nil
	}))
	t.Cleanup(stubServerRedirectPromptReader(strings.NewReader("")))

	code := runServerRedirectRemove(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectRemoveOptions{
		CIDR: "10.10.0.0/16",
	})
	if code != 0 {
		t.Fatalf("runServerRedirectRemove exit = %d, want 0", code)
	}
}

func TestRunServerRedirectRemove_MultipleMatchingRedirectsPrompts(t *testing.T) {
	t.Cleanup(stubServerRedirectRemove(func(opts server.RedirectRemoveOptions) error {
		if opts.Tag != "beta.rev" || opts.Hostname != "edge-b" {
			t.Fatalf("binding mismatch: got %s/%s want beta.rev/edge-b", opts.Tag, opts.Hostname)
		}
		return nil
	}))
	t.Cleanup(stubServerRedirectList(func(server.RedirectListOptions) ([]server.RedirectRecord, error) {
		return []server.RedirectRecord{
			{Type: "CIDR", Value: "10.10.0.0/16", CIDR: "10.10.0.0/16", Tag: "alpha.rev", Hostname: "edge-a"},
			{Type: "CIDR", Value: "10.10.0.0/16", CIDR: "10.10.0.0/16", Tag: "beta.rev", Hostname: "edge-b"},
		}, nil
	}))
	t.Cleanup(stubServerRedirectPromptReader(strings.NewReader("2\n")))

	code := runServerRedirectRemove(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectRemoveOptions{
		CIDR: "10.10.0.0/16",
	})
	if code != 0 {
		t.Fatalf("runServerRedirectRemove exit = %d, want 0", code)
	}
}

func TestRunServerRedirectToggle_SingleMatchingRedirectSkipsPrompt(t *testing.T) {
	t.Cleanup(stubServerRedirectToggle(func(opts server.RedirectSetEnabledOptions) error {
		if opts.Tag != "alpha.rev" || opts.Hostname != "edge-a" || !opts.Enabled {
			t.Fatalf("toggle options mismatch: %+v", opts)
		}
		return nil
	}))
	t.Cleanup(stubServerRedirectList(func(server.RedirectListOptions) ([]server.RedirectRecord, error) {
		return []server.RedirectRecord{
			{Type: "CIDR", Value: "10.10.0.0/16", CIDR: "10.10.0.0/16", Tag: "alpha.rev", Hostname: "edge-a"},
		}, nil
	}))
	t.Cleanup(stubServerRedirectPromptReader(strings.NewReader("")))

	code := runServerRedirectToggle(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectToggleOptions{
		CIDR: "10.10.0.0/16",
	}, true)
	if code != 0 {
		t.Fatalf("runServerRedirectToggle exit = %d, want 0", code)
	}
}

func TestRunServerRedirectToggle_MultipleMatchingRedirectsPrompts(t *testing.T) {
	t.Cleanup(stubServerRedirectToggle(func(opts server.RedirectSetEnabledOptions) error {
		if opts.Tag != "beta.rev" || opts.Hostname != "edge-b" || opts.Enabled {
			t.Fatalf("toggle options mismatch: %+v", opts)
		}
		return nil
	}))
	t.Cleanup(stubServerRedirectList(func(server.RedirectListOptions) ([]server.RedirectRecord, error) {
		return []server.RedirectRecord{
			{Type: "CIDR", Value: "10.10.0.0/16", CIDR: "10.10.0.0/16", Tag: "alpha.rev", Hostname: "edge-a"},
			{Type: "CIDR", Value: "10.10.0.0/16", CIDR: "10.10.0.0/16", Tag: "beta.rev", Hostname: "edge-b"},
		}, nil
	}))
	t.Cleanup(stubServerRedirectPromptReader(strings.NewReader("2\n")))

	code := runServerRedirectToggle(context.Background(), serverCfg("C:\\srv", "cfg", ""), serverRedirectToggleOptions{
		CIDR: "10.10.0.0/16",
	}, false)
	if code != 0 {
		t.Fatalf("runServerRedirectToggle exit = %d, want 0", code)
	}
}
