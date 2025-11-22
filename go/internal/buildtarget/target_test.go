package buildtarget

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTargetNamingHelpers(t *testing.T) {
	tgt := Target{
		GOOS:      "linux",
		GOARCH:    "arm64",
		Archive:   ArchiveTarGz,
		BinaryExt: "",
		OutDir:    "",
	}
	if id := tgt.Identifier(); id != "linux-arm64" {
		t.Fatalf("Identifier mismatch: %s", id)
	}
	if name := tgt.BinaryName("xp2p"); name != "xp2p" {
		t.Fatalf("BinaryName mismatch: %s", name)
	}
	if path := tgt.OutputDir("dist"); path != filepath.Join("dist", "linux-arm64") {
		t.Fatalf("OutputDir mismatch: %s", path)
	}
	if archive := tgt.ArchiveName("xp2p", "0.1.0"); archive != "xp2p-0.1.0-linux-arm64.tar.gz" {
		t.Fatalf("ArchiveName mismatch: %s", archive)
	}
	if latest := tgt.LatestArchiveName("xp2p"); latest != "xp2p-latest-linux-arm64.tar.gz" {
		t.Fatalf("LatestArchiveName mismatch: %s", latest)
	}
}

func TestTargetsFiltersAndLookup(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatalf("expected default targets")
	}
	release := ReleaseTargets()
	for _, target := range release {
		if !target.release {
			t.Fatalf("ReleaseTargets contains non-release entries")
		}
	}

	tgt, ok := Lookup("LINUX", "ARM64")
	if !ok {
		t.Fatalf("Lookup should find linux/arm64 target")
	}
	if tgt.GOOS != "linux" || tgt.GOARCH != "arm64" {
		t.Fatalf("Lookup returned unexpected target %+v", tgt)
	}

	if _, ok := Lookup("plan9", "amd64"); ok {
		t.Fatalf("Lookup unexpectedly succeeded")
	}
}

func TestXrayDependency(t *testing.T) {
	dep := xrayDependency("linux", "amd64", "xray", true)
	if !strings.Contains(dep.Source, "distro/linux/bundle/amd64/xray") {
		t.Fatalf("unexpected dependency source %s", dep.Source)
	}
	if dep.Destination != "xray" || !dep.Optional {
		t.Fatalf("unexpected dependency fields: %+v", dep)
	}
}
