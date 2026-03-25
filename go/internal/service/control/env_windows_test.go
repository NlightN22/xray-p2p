//go:build windows

package control

import "testing"

func TestMergeServiceEnv(t *testing.T) {
	existing := []string{"FOO=1", "BAR=2", "INVALID"}
	vars := map[string]string{
		"BAR": "updated",
		"BAZ": "3",
	}

	got := mergeServiceEnv(existing, vars)
	want := []string{"FOO=1", "BAR=updated", "INVALID", "BAZ=3"}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}
