//go:build linux

package natredirect

import (
	"path/filepath"
	"testing"
)

func TestListEntryDirUsesConfigRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)

	want := filepath.Join(root, "nftables", "xray-transparent.d")
	if got := listEntryDir(); got != want {
		t.Fatalf("list entry dir = %q, want %q", got, want)
	}
}
