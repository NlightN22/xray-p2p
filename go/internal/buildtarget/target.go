package buildtarget

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ArchiveFormat describes how release artifacts are packaged.
type ArchiveFormat string

const (
	// ArchiveTarGz represents a gzip-compressed tarball.
	ArchiveTarGz ArchiveFormat = "tar.gz"
	// ArchiveZip represents a zip archive.
	ArchiveZip ArchiveFormat = "zip"
)

// Target captures a single GOOS/GOARCH build along with packaging metadata.
type Target struct {
	GOOS      string
	GOARCH    string
	Archive   ArchiveFormat
	BinaryExt string
	GoEnv     map[string]string
	OutDir    string
	release   bool
}

// Identifier returns a stable shorthand used in artifact names.
func (t Target) Identifier() string {
	return fmt.Sprintf("%s-%s", t.GOOS, t.GOARCH)
}

// OutputDir returns the per-target directory name under the provided base.
func (t Target) OutputDir(base string) string {
	dir := t.OutDir
	if strings.TrimSpace(dir) == "" {
		dir = t.Identifier()
	}
	return filepath.Join(base, dir)
}

// BinaryName returns the name of the compiled binary for the provided base name.
func (t Target) BinaryName(base string) string {
	return base + t.BinaryExt
}

// ArchiveName returns the packaged archive name for the provided base name and version.
func (t Target) ArchiveName(base, version string) string {
	return fmt.Sprintf("%s-%s-%s.%s", base, version, t.Identifier(), t.Archive)
}

// LatestArchiveName returns the packaged archive name for the rolling "latest" release.
func (t Target) LatestArchiveName(base string) string {
	return fmt.Sprintf("%s-latest-%s.%s", base, t.Identifier(), t.Archive)
}

// ReleaseTargets returns the list of targets that should produce release artifacts.
func ReleaseTargets() []Target {
	return filterTargets(func(t Target) bool { return t.release })
}

// All returns the complete set of known build targets.
func All() []Target {
	return append([]Target(nil), targets...)
}

// Lookup finds a target by GOOS/GOARCH.
func Lookup(goos, goarch string) (Target, bool) {
	for _, t := range targets {
		if strings.EqualFold(t.GOOS, goos) && strings.EqualFold(t.GOARCH, goarch) {
			return t, true
		}
	}
	return Target{}, false
}

func filterTargets(pred func(Target) bool) []Target {
	var out []Target
	for _, t := range targets {
		if pred(t) {
			out = append(out, t)
		}
	}
	return out
}

var targets = []Target{
	{
		GOOS:      "windows",
		GOARCH:    "amd64",
		Archive:   ArchiveZip,
		BinaryExt: ".exe",
		OutDir:    "windows-amd64",
		release:   true,
	},
	{
		GOOS:    "linux",
		GOARCH:  "amd64",
		Archive: ArchiveTarGz,
		OutDir:  "linux-amd64",
		release: true,
	},
	{
		GOOS:    "linux",
		GOARCH:  "arm64",
		Archive: ArchiveTarGz,
		OutDir:  "linux-arm64",
		release: true,
	},
	{
		GOOS:    "linux",
		GOARCH:  "mipsle",
		Archive: ArchiveTarGz,
		GoEnv: map[string]string{
			"GOMIPS": "softfloat",
		},
		OutDir: "linux-mipsle-softfloat",
	},
}
