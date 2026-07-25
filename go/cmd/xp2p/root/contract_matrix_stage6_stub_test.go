//go:build !linux

package root

import "testing"

func registerStage6PlatformContractCases(map[string]contractCase) {}

func TestStage6LinuxLeavesAbsentFromNonLinuxTree(t *testing.T) {
	leaves := jsonLeafPaths(NewCommand())
	for _, path := range stage6Paths {
		if leaves[path] {
			t.Errorf("Linux-only command unexpectedly exists in non-Linux Cobra tree: %s", path)
		}
	}
}
