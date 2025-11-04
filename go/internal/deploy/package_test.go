package deploy

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	expectedName := filepath.Join(outDir, "xp2p-client-1.2.3-10.0.10.10-20241123-184510.zip")
	if path != expectedName {
		t.Fatalf("archive path mismatch: expected %q, got %q", expectedName, path)
	}

	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()

	files := make(map[string]*zip.File)
	for _, file := range reader.File {
		files[file.Name] = file
	}

	script := files["templates/windows-amd64/install.ps1"]
	if script == nil {
		t.Fatalf("expected install script in archive")
	}
	data := readZipFile(t, script)
	if !strings.Contains(string(data), "placeholder install script") {
		t.Fatalf("unexpected script content: %q", string(data))
	}

	configEntry := files["config/deployment.json"]
	if configEntry == nil {
		t.Fatalf("expected deployment.json in archive")
	}
	configData := readZipFile(t, configEntry)

	var payload map[string]string
	if err := json.Unmarshal(configData, &payload); err != nil {
		t.Fatalf("unmarshal deployment.json: %v", err)
	}

	if payload["remote_host"] != "10.0.10.10" {
		t.Fatalf("remote_host mismatch: %q", payload["remote_host"])
	}
	if payload["xp2p_version"] != "1.2.3" {
		t.Fatalf("xp2p_version mismatch: %q", payload["xp2p_version"])
	}
	if payload["generated_at"] != timestamp.Format(time.RFC3339) {
		t.Fatalf("generated_at mismatch: %q", payload["generated_at"])
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
		t.Fatalf("expected sanitized host in archive name, got %q", name)
	}
}

func readZipFile(t *testing.T, file *zip.File) []byte {
	t.Helper()

	reader, err := file.Open()
	if err != nil {
		t.Fatalf("open zip entry %q: %v", file.Name, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read zip entry %q: %v", file.Name, err)
	}
	return data
}
