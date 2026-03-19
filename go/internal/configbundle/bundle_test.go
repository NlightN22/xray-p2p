package configbundle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultArchiveName(t *testing.T) {
	name := DefaultArchiveName("client", FormatZip, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))
	if name != "xp2p-client-backup-20240102-030405.zip" {
		t.Fatalf("unexpected archive name: %s", name)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		format Format
		ext    string
	}{
		{name: "zip", format: FormatZip, ext: ".zip"},
		{name: "tar.gz", format: FormatTarGz, ext: ".tar.gz"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Helper()
			tmp := t.TempDir()
			sourceRoot := filepath.Join(tmp, "root")
			files := createFixtureRoot(t, sourceRoot)
			archive := filepath.Join(tmp, "backup"+test.ext)

			if err := ExportConfigRoot(sourceRoot, archive); err != nil {
				t.Fatalf("export failed: %v", err)
			}

			destRoot := filepath.Join(tmp, "dest")
			if err := os.MkdirAll(destRoot, 0o755); err != nil {
				t.Fatalf("create dest root: %v", err)
			}
			oldMarker := filepath.Join(destRoot, "old.txt")
			if err := os.WriteFile(oldMarker, []byte("old"), 0o644); err != nil {
				t.Fatalf("write old marker: %v", err)
			}

			if err := ImportConfigRoot(destRoot, archive); err != nil {
				t.Fatalf("import failed: %v", err)
			}

			assertFixtureRoot(t, destRoot, files)

			backups, err := filepath.Glob(destRoot + ".bak-*")
			if err != nil {
				t.Fatalf("glob backup: %v", err)
			}
			if len(backups) == 0 {
				t.Fatalf("expected backup path")
			}
			if _, err := os.Stat(filepath.Join(backups[0], "old.txt")); err != nil {
				t.Fatalf("expected old marker in backup: %v", err)
			}
		})
	}
}

func TestImportRejectsPathTraversal(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create root: %v", err)
	}
	marker := filepath.Join(root, "marker.txt")
	if err := os.WriteFile(marker, []byte("marker"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	archive := filepath.Join(tmp, "bad.zip")
	if err := writeZipWithEntry(archive, "../evil.txt", "nope"); err != nil {
		t.Fatalf("write bad zip: %v", err)
	}

	if err := ImportConfigRoot(root, archive); err == nil {
		t.Fatalf("expected import error")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker missing after failed import: %v", err)
	}

	backups, err := filepath.Glob(root + ".bak-*")
	if err != nil {
		t.Fatalf("glob backup: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("unexpected backup on failed import")
	}
}

func createFixtureRoot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{
		filepath.Join("config-client", "inbounds.json"):        `{"in":1}`,
		filepath.Join("config-client", "nested", "route.json"): `{"route":true}`,
		filepath.Join("config-server", "inbounds.json"):        `{"in":2}`,
		"xp2p-client.toml":          "client=1\n",
		"xp2p-server.toml":          "server=1\n",
		"cert.pem":                  "cert\n",
		"key.pem":                   "key\n",
		"xp2p-client.state.json":    `{"state":"c"}`,
		"xp2p-server.state.json":    `{"state":"s"}`,
		"install-state-client.json": `{"install":"c"}`,
		"install-state-server.json": `{"install":"s"}`,
	}

	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return files
}

func assertFixtureRoot(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, rel)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if strings.TrimSpace(string(got)) != strings.TrimSpace(content) {
			t.Fatalf("content mismatch for %s", rel)
		}
	}
}

func writeZipWithEntry(path, name, content string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	header := &zip.FileHeader{
		Name: name,
	}
	w, err := writer.CreateHeader(header)
	if err != nil {
		_ = writer.Close()
		return err
	}
	if _, err := w.Write([]byte(content)); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}
