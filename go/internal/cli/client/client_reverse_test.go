package clientcmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
)

func TestRunClientReverseListPrintsRecords(t *testing.T) {
	cfg := clientCfg("C:\\xp2p", "cfg-client")
	restore := stubClientReverseList(func(opts client.ReverseListOptions) ([]client.ReverseRecord, error) {
		requireEqual(t, opts.InstallDir, `D:\xp2p`, "install dir")
		requireEqual(t, opts.ConfigDir, "conf", "config dir")
		return []client.ReverseRecord{
			{
				Tag:         "reverse-alpha.rev",
				Host:        "edge.example.com",
				User:        "alpha@example.com",
				EndpointTag: "proxy-edge",
				Bridge:      true,
				DirectRule:  true,
			},
		}, nil
	})
	defer restore()

	output := captureStdout(t, func() {
		code := runClientReverseList(context.Background(), cfg, []string{"--path", `D:\xp2p`, "--config-dir", "conf"})
		requireEqual(t, code, 0, "exit code")
	})
	if !strings.Contains(output, "reverse-alpha.rev") {
		t.Fatalf("xp2p client reverse list output missing tag: %q", output)
	}
	if !strings.Contains(output, "ROUTING-BRIDGE") || !strings.Contains(output, "DIRECT RULE") {
		t.Fatalf("xp2p client reverse list output missing headers: %q", output)
	}
	if !strings.Contains(output, "present") {
		t.Fatalf("xp2p client reverse list output missing status label: %q", output)
	}
}

func TestRunClientReverseListHandlesErrors(t *testing.T) {
	cfg := clientCfg("C:\\xp2p", "cfg-client")
	restore := stubClientReverseList(func(client.ReverseListOptions) ([]client.ReverseRecord, error) {
		return nil, errors.New("boom")
	})
	defer restore()

	code := runClientReverseList(context.Background(), cfg, nil)
	if code != 1 {
		t.Fatalf("xp2p client reverse list exit code = %d, want 1", code)
	}
}
