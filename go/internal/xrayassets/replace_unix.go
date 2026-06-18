//go:build !windows

package xrayassets

import "os"

func replaceFile(tmpPath, target string) error {
	return os.Rename(tmpPath, target)
}
