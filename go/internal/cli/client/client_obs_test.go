package clientcmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/xrayapi"
)

func TestClientObsCommandRegisteredWithoutAlias(t *testing.T) {
	cmd := NewCommand(func() config.Config { return config.Config{} })
	obsCmd, _, err := cmd.Find([]string{"obs"})
	if err != nil {
		t.Fatalf("find obs command: %v", err)
	}
	if obsCmd == nil || obsCmd.Use != "obs" {
		t.Fatalf("unexpected obs command: %+v", obsCmd)
	}
	if len(obsCmd.Aliases) != 0 {
		t.Fatalf("expected no aliases, got %v", obsCmd.Aliases)
	}
}

func TestRunClientObsReadsLiveAPIAndRendersStatuses(t *testing.T) {
	dir := t.TempDir()
	liveDir := filepath.Join(dir, layout.StateDirName, layout.LiveDirName, layout.ClientConfigDir)
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("mkdir live dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, layout.XrayConfigFileName), []byte(`{"api":{"listen":"127.0.0.1:19091"}}`), 0o644); err != nil {
		t.Fatalf("write live xray config: %v", err)
	}

	var gotAddress string
	oldStatuses := clientObsStatusesFunc
	clientObsStatusesFunc = func(_ context.Context, opts xrayapi.ObservatoryOptions) ([]xrayapi.OutboundObservation, error) {
		gotAddress = opts.Address
		return []xrayapi.OutboundObservation{
			{Tag: "proxy-alpha", Alive: true, DelayMillis: 42, LastTryUnix: 100, LastSeenUnix: 120},
			{Tag: "proxy-beta", Alive: false, LastError: "timeout"},
		}, nil
	}
	t.Cleanup(func() { clientObsStatusesFunc = oldStatuses })

	output := captureStdout(t, func() {
		code := runClientObs(context.Background(), config.Config{}, clientObsOptions{Path: dir})
		if code != 0 {
			t.Fatalf("runClientObs returned %d", code)
		}
	})
	if gotAddress != "127.0.0.1:19091" {
		t.Fatalf("address = %q, want 127.0.0.1:19091", gotAddress)
	}
	if !strings.Contains(output, "proxy-alpha") || !strings.Contains(output, "42ms") || !strings.Contains(output, "timeout") {
		t.Fatalf("unexpected output:\n%s", output)
	}
}
