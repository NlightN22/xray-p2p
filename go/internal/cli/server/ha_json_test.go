package servercmd

import (
	"context"
	"encoding/json"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestHAStatusPublishesEmptyTypedState(t *testing.T) {
	t.Setenv("XP2P_CONFIG_ROOT", t.TempDir())
	ctx, captured := clioutput.CaptureResult(context.Background())
	cmd := newServerHACmd(func() config.Config { return config.Config{} })
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(captured())
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Configured bool `json:"configured"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Configured {
		t.Fatal("empty HA state reported configured")
	}
}
