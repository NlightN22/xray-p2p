package servercmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerReverseListPrintsRecords(t *testing.T) {
	cfg := serverCfg("C:\\xp2p", "cfg-server", "edge.example.com")
	restore := stubServerReverseList(func(opts server.ReverseListOptions) ([]server.ReverseRecord, error) {
		requireEqual(t, opts.InstallDir, `D:\xp2p`, "install dir")
		requireEqual(t, opts.ConfigDir, "conf", "config dir")
		return []server.ReverseRecord{
			{
				Domain:      "reverse-alpha.rev",
				Host:        "edge.example.com",
				User:        "alpha@example.com",
				Tag:         "reverse-alpha.rev",
				Portal:      true,
				RoutingRule: true,
			},
		}, nil
	})
	defer restore()

	output := captureStdout(t, func() {
		code := runServerReverseList(context.Background(), cfg, []string{"--path", `D:\xp2p`, "--config-dir", "conf"})
		requireEqual(t, code, 0, "exit code")
	})
	if !strings.Contains(output, "reverse-alpha.rev") {
		t.Fatalf("xp2p server reverse list output missing tag: %q", output)
	}
	if !strings.Contains(output, "PORTAL") || !strings.Contains(output, "ROUTING RULE") {
		t.Fatalf("xp2p server reverse list output missing headers: %q", output)
	}
	if !strings.Contains(output, "present") {
		t.Fatalf("xp2p server reverse list output missing status label: %q", output)
	}
}

func TestRunServerReverseListHandlesErrors(t *testing.T) {
	cfg := serverCfg("C:\\xp2p", "cfg-server", "edge.example.com")
	restore := stubServerReverseList(func(server.ReverseListOptions) ([]server.ReverseRecord, error) {
		return nil, errors.New("boom")
	})
	defer restore()

	code := runServerReverseList(context.Background(), cfg, nil)
	if code != 1 {
		t.Fatalf("xp2p server reverse list exit code = %d, want 1", code)
	}
}
