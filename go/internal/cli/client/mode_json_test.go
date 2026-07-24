package clientcmd

import (
	"context"
	"encoding/json"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestClientModeQueryPublishesTypedResult(t *testing.T) {
	t.Setenv("XP2P_CONFIG_ROOT", t.TempDir())
	ctx, captured := clioutput.CaptureResult(context.Background())
	cfg := config.Config{}
	cfg.Client.TunEnabled = true
	cfg.Client.TunMode = "split"
	if code := runClientMode(ctx, cfg, nil); code != 0 {
		t.Fatalf("exit code=%d", code)
	}
	data, err := json.Marshal(captured())
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Mode          string `json:"mode"`
		TunMode       string `json:"tun_mode"`
		TunModeStatus string `json:"tun_mode_status"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Mode != "tun" || result.TunMode != "split" || result.TunModeStatus != "" {
		t.Fatalf("result=%+v", result)
	}
}
