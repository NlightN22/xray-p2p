//go:build windows

package xrayassets

import "golang.org/x/sys/windows"

func replaceFile(tmpPath, target string) error {
	return windows.MoveFileEx(
		windows.StringToUTF16Ptr(tmpPath),
		windows.StringToUTF16Ptr(target),
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}
