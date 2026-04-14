package server

import (
	"testing"
)

func mustPendingConfigDir(t *testing.T, liveConfigDir string) string {
	t.Helper()
	dir, err := pendingConfigDir(liveConfigDir)
	if err != nil {
		t.Fatalf("pending config dir: %v", err)
	}
	return dir
}
