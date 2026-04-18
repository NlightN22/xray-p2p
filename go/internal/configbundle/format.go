package configbundle

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

type Format string

const (
	FormatZip   Format = "zip"
	FormatTarGz Format = "tar.gz"
)

func DefaultArchiveFormat() Format {
	if runtime.GOOS == "windows" {
		return FormatZip
	}
	return FormatTarGz
}

func DetectArchiveFormat(path string) (Format, error) {
	lower := strings.ToLower(strings.TrimSpace(path))
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return FormatZip, nil
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return FormatTarGz, nil
	default:
		return "", fmt.Errorf("configbundle: unsupported archive format: %s", path)
	}
}

func DefaultArchiveName(role string, format Format, now time.Time) string {
	trimmedRole := strings.TrimSpace(strings.ToLower(role))
	if trimmedRole == "" {
		trimmedRole = "bundle"
	}
	ts := now.UTC().Format("20060102-150405")
	base := fmt.Sprintf("xp2p-%s-backup-%s", trimmedRole, ts)
	switch format {
	case FormatZip:
		return base + ".zip"
	case FormatTarGz:
		return base + ".tar.gz"
	default:
		return base
	}
}
