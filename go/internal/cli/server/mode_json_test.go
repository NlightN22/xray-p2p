package servercmd

import (
	"context"
	"encoding/json"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestServerModeQueryPublishesTypedResult(t *testing.T) {
	t.Setenv("XP2P_CONFIG_ROOT", t.TempDir())
	ctx, captured := clioutput.CaptureResult(context.Background())
	cfg := config.Config{}
	cfg.Server.TunEnabled = true
	if code := runServerMode(ctx, cfg, nil); code != 0 {
		t.Fatalf("exit code=%d", code)
	}
	data, err := json.Marshal(captured())
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Mode != "tun" {
		t.Fatalf("mode=%q", result.Mode)
	}
}
