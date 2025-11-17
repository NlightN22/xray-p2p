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
