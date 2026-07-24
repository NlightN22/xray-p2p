//go:build linux

package natredirect

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestAddPrintOnlyPublishesTypedPlanWithoutPrompt(t *testing.T) {
	ctx, captured := clioutput.CaptureResult(context.Background())
	cmd := newAddCmd(func() config.Config { return config.Config{} })
	cmd.SetContext(ctx)
	tmp := t.TempDir()
	cmd.SetArgs([]string{
		"--cidr", "10.10.0.0/16",
		"--port", "12345",
		"--print-only",
		"--snippet", filepath.Join(tmp, "redirect.nft"),
		"--entry-dir", filepath.Join(tmp, "entries"),
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(captured())
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		SnippetPath string   `json:"snippet_path"`
		IPTables    []string `json:"iptables"`
		Entry       *struct {
			CIDR string `json:"cidr"`
			Port int    `json:"port"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Entry == nil || result.Entry.CIDR != "10.10.0.0/16" || result.Entry.Port != 12345 {
		t.Fatalf("plan=%+v", result)
	}
	if result.IPTables == nil {
		t.Fatal("iptables must be an empty array, not null")
	}
}
