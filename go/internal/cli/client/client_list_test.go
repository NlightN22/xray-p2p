package clientcmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestRunClientListSuccess(t *testing.T) {
	restore := stubClientList(func(opts client.ListOptions) ([]client.EndpointRecord, error) {
		if opts.InstallDir != `C:\xp2p` || opts.ConfigDir != "cfg" {
			t.Fatalf("unexpected list options: %+v", opts)
		}
		return []client.EndpointRecord{
			{Hostname: "alpha", Tag: "alpha-tag", Address: "edge.example", Port: 62022, User: "alpha@example.com", AllowInsecure: false, ServerName: "edge.example"},
		}, nil
	})
	t.Cleanup(restore)

	cfg := config.Config{
		Client: config.ClientConfig{
			InstallDir: `C:\xp2p`,
			ConfigDir:  "cfg",
		},
	}

	output := captureStdout(t, func() {
		code := runClientList(context.Background(), cfg, nil)
		if code != 0 {
			t.Fatalf("runClientList exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(output, "HOSTNAME") || !strings.Contains(output, "alpha-tag") {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestRunClientListError(t *testing.T) {
	wantErr := errors.New("list failure")
	restore := stubClientList(func(client.ListOptions) ([]client.EndpointRecord, error) {
		return nil, wantErr
	})
	t.Cleanup(restore)

	cfg := config.Config{}
	code := runClientList(context.Background(), cfg, []string{"--path", `D:\xp2p`, "--config-dir", "cfg"})
	if code != 1 {
		t.Fatalf("runClientList exit code = %d, want 1", code)
	}
}

func TestRunClientListLinks(t *testing.T) {
	restore := stubClientList(func(opts client.ListOptions) ([]client.EndpointRecord, error) {
		if !opts.Pending {
			t.Fatalf("expected pending list options: %+v", opts)
		}
		return []client.EndpointRecord{
			{Hostname: "alpha", Link: "trojan://secret@example.test:443?security=tls&sni=example.test#alpha"},
			{Hostname: "beta"},
		}, nil
	})
	t.Cleanup(restore)

	output := captureStdout(t, func() {
		code := runClientList(context.Background(), config.Config{}, []string{"--pending", "--link"})
		if code != 0 {
			t.Fatalf("runClientList exit code = %d, want 0", code)
		}
	})
	if strings.TrimSpace(output) != "trojan://secret@example.test:443?security=tls&sni=example.test#alpha" {
		t.Fatalf("unexpected link output: %q", output)
	}
}
