package deploy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
)

func TestBuildPackageCreatesArchive(t *testing.T) {
	outDir := t.TempDir()
	timestamp := time.Date(2024, 11, 23, 18, 45, 10, 0, time.UTC)

	path, err := BuildPackage(PackageOptions{
		RemoteHost: "10.0.10.10",
		OutputDir:  outDir,
		Version:    "1.2.3",
		Timestamp:  timestamp,
	})
	if err != nil {
		t.Fatalf("BuildPackage: %v", err)
	}

	expectedName := filepath.Join(outDir, "xp2p-client-1.2.3-10.0.10.10-20241123-184510")
	if path != expectedName {
		t.Fatalf("package path mismatch: expected %q, got %q", expectedName, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat package: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected package to be directory")
	}

	scriptPath := filepath.Join(path, "templates", "windows-amd64", "install.ps1")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("expected install script in archive")
	}
	if !strings.Contains(string(data), "placeholder install script") {
		t.Fatalf("unexpected script content: %q", string(data))
	}

	configPath := filepath.Join(path, "config", "deployment.json")
	configFile, err := os.Open(configPath)
	if err != nil {
		t.Fatalf("open deployment.json: %v", err)
	}
	defer configFile.Close()

	manifest, err := spec.Read(configFile)
	if err != nil {
		t.Fatalf("unmarshal deployment.json: %v", err)
	}

	if manifest.RemoteHost != "10.0.10.10" {
		t.Fatalf("remote_host mismatch: %q", manifest.RemoteHost)
	}
	if manifest.XP2PVersion != "1.2.3" {
		t.Fatalf("xp2p_version mismatch: %q", manifest.XP2PVersion)
	}
	if manifest.GeneratedAt != timestamp {
		t.Fatalf("generated_at mismatch: %v", manifest.GeneratedAt)
	}
}

func TestBuildPackageRejectsEmptyHost(t *testing.T) {
	_, err := BuildPackage(PackageOptions{})
	if !errors.Is(err, ErrRemoteHostEmpty) {
		t.Fatalf("expected ErrRemoteHostEmpty, got %v", err)
	}
}

func TestBuildPackageSanitizesArchiveName(t *testing.T) {
	outDir := t.TempDir()
	timestamp := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)

	path, err := BuildPackage(PackageOptions{
		RemoteHost: "..Bad host??",
		OutputDir:  outDir,
		Version:    "0.9.0",
		Timestamp:  timestamp,
	})
	if err != nil {
		t.Fatalf("BuildPackage: %v", err)
	}

	name := filepath.Base(path)
	if !strings.Contains(name, "bad-host") {
		t.Fatalf("expected sanitized host in directory name, got %q", name)
	}
}
