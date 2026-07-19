//go:build linux || windows

package client

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestRuntimeCommitRejectsStaleDesiredSnapshot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	path := filepath.Join(root, layout.ClientConfigFileName)
	if err := os.WriteFile(path, []byte("[client]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := loadClientInstallState(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[client]\ntun_enabled = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = commitClientRuntimeStateResult(context.Background(), state)
	if err == nil || !strings.Contains(err.Error(), "changed concurrently") {
		t.Fatalf("error = %v, want concurrent Desired change", err)
	}
}
