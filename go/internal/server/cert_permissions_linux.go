//go:build linux

package server

import (
	"os"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func warnIfKeyTooPermissive(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		return
	}
	if info.Mode().Perm()&0o004 != 0 {
		logging.Warn("key file is world-readable", "path", path)
	}
}
