package deploy

import (
	"bytes"
	"encoding/json"
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
		InstallDir: `C:\xp2p`,
		TrojanPort: "8445",
		TrojanUser: "client@example.invalid",
		TrojanPass: "secret",
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
	scriptContent := string(data)
	for _, fragment := range []string{
		"Build-ArtifactUrl",
		"xp2p-$($manifest.Version)-windows-amd64.zip",
		"--deploy-file",
		"--port",
		"XP2P_RELEASE_BASE_URL",
		"$manifest.InstallDir",
		"$manifest.TrojanPort",
	} {
		if !strings.Contains(scriptContent, fragment) {
			t.Fatalf("install.ps1 is missing expected fragment %q", fragment)
		}
	}

	configPath := filepath.Join(path, "config", "deployment.json")
	manifestBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read deployment.json: %v", err)
	}

	var manifestFields map[string]any
	if err := json.Unmarshal(manifestBytes, &manifestFields); err != nil {
		t.Fatalf("manifest json decode: %v", err)
	}
	expectedKeys := []string{
		"remote_host",
		"xp2p_version",
		"generated_at",
		"install_dir",
		"trojan_port",
		"trojan_user",
		"trojan_password",
	}
	if len(manifestFields) != len(expectedKeys) {
		t.Fatalf("manifest contains unexpected keys: %#v", manifestFields)
	}
	for _, key := range expectedKeys {
		if _, ok := manifestFields[key]; !ok {
			t.Fatalf("manifest missing key %q", key)
		}
	}

	configFile := bytes.NewReader(manifestBytes)
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
	if manifest.InstallDir != `C:\xp2p` {
		t.Fatalf("install_dir mismatch: %q", manifest.InstallDir)
	}
	if manifest.TrojanPort != "8445" {
		t.Fatalf("trojan_port mismatch: %q", manifest.TrojanPort)
	}
	if manifest.TrojanUser != "client@example.invalid" {
		t.Fatalf("trojan_user mismatch: %q", manifest.TrojanUser)
	}
	if manifest.TrojanPassword != "secret" {
		t.Fatalf("trojan_password mismatch: %q", manifest.TrojanPassword)
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
		InstallDir: `C:\xp2p`,
		TrojanPort: "62022",
		TrojanUser: "user@example.com",
		TrojanPass: "secret",
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
