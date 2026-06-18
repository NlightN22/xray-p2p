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
			{Tag: "proxy-b", Hostname: "edge-b"},
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

func TestClientRedirectCommandListsRedirects(t *testing.T) {
	called := 0
	t.Cleanup(stubClientRedirectList(func(opts client.RedirectListOptions) ([]client.RedirectRecord, error) {
		called++
		if opts.InstallDir != `D:\xp2p` {
			t.Fatalf("InstallDir mismatch: got %s want D:\\xp2p", opts.InstallDir)
		}
		if opts.ConfigDir != "client-a" {
			t.Fatalf("ConfigDir mismatch: got %s want client-a", opts.ConfigDir)
		}
		if !opts.Pending {
			t.Fatalf("Pending mismatch: got false want true")
		}
		return []client.RedirectRecord{
			{Type: "cidr", Value: "10.20.0.0/24", Tag: "proxy-a", Hostname: "edge-a"},
		}, nil
	}))

	output := captureStdout(t, func() {
		code := Execute(
			context.Background(),
			clientCfg(`C:\xp2p`, "cfg"),
			[]string{"redirect", "--path", `D:\xp2p`, "--config-dir", "client-a", "--pending"},
		)
		if code != 0 {
			t.Fatalf("Execute exit = %d, want 0", code)
		}
	})
	if called != 1 {
		t.Fatalf("clientRedirectListFunc called %d times, want 1", called)
	}
	if !strings.Contains(output, "TYPE") || !strings.Contains(output, "10.20.0.0/24") {
		t.Fatalf("redirect output = %q, want list table", output)
	}
}
