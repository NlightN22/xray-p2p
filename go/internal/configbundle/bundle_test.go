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

func TestRoleExportImportRoundTrip(t *testing.T) {
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
			tmp := t.TempDir()
			sourceRoot := filepath.Join(tmp, "root")
			createFixtureRoot(t, sourceRoot)

			destRoot := filepath.Join(tmp, "dest")
			if err := os.MkdirAll(destRoot, 0o755); err != nil {
				t.Fatalf("create dest root: %v", err)
			}
			oldMarker := filepath.Join(destRoot, "old.txt")
			if err := os.WriteFile(oldMarker, []byte("old"), 0o644); err != nil {
				t.Fatalf("write old marker: %v", err)
			}

			t.Run("client", func(t *testing.T) {
				archive := filepath.Join(tmp, "client"+test.ext)
				if err := ExportRoleConfigRoot("client", sourceRoot, archive); err != nil {
					t.Fatalf("export failed: %v", err)
				}
				if err := ImportRoleConfigRoot("client", destRoot, archive); err != nil {
					t.Fatalf("import failed: %v", err)
				}

				assertFileContains(t, filepath.Join(destRoot, "xp2p-client.toml"), "client=1")
				assertFileContains(t, filepath.Join(destRoot, "config-client", "nested", "route.json"), `"route":true`)
				assertPathMissing(t, filepath.Join(destRoot, "xp2p-server.toml"))
				assertPathMissing(t, filepath.Join(destRoot, "config-server", "inbounds.json"))
				assertPathMissing(t, filepath.Join(destRoot, "tls", "server", "cert.pem"))
				assertPathExists(t, filepath.Join(destRoot, ".state", "apply.request"))
				assertFileContains(t, oldMarker, "old")
			})

			t.Run("server", func(t *testing.T) {
				dest2 := filepath.Join(tmp, "dest-server")
				if err := os.MkdirAll(dest2, 0o755); err != nil {
					t.Fatalf("create dest root: %v", err)
				}

				archive := filepath.Join(tmp, "server"+test.ext)
				if err := ExportRoleConfigRoot("server", sourceRoot, archive); err != nil {
					t.Fatalf("export failed: %v", err)
				}
				if err := ImportRoleConfigRoot("server", dest2, archive); err != nil {
					t.Fatalf("import failed: %v", err)
				}

				assertFileContains(t, filepath.Join(dest2, "xp2p-server.toml"), "server=1")
				assertFileContains(t, filepath.Join(dest2, "config-server", "inbounds.json"), `{"in":2}`)
				assertFileContains(t, filepath.Join(dest2, "tls", "server", "cert.pem"), "cert")
				assertPathMissing(t, filepath.Join(dest2, "xp2p-client.toml"))
				assertPathMissing(t, filepath.Join(dest2, "config-client", "inbounds.json"))
				assertPathExists(t, filepath.Join(dest2, ".state", "apply.request"))
			})
		})
	}
}

func TestRoleImportRejectsPathTraversal(t *testing.T) {
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

	if err := ImportRoleConfigRoot("client", root, archive); err == nil {
		t.Fatalf("expected import error")
	}
	assertFileContains(t, marker, "marker")
	assertPathMissing(t, filepath.Join(root, ".state", "apply.request"))
}

func createFixtureRoot(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		filepath.Join("config-client", "inbounds.json"):        `{"in":1}`,
		filepath.Join("config-client", "nested", "route.json"): `{"route":true}`,
		filepath.Join("config-server", "inbounds.json"):        `{"in":2}`,
		"xp2p-client.toml":          "client=1\n",
		"xp2p-server.toml":          "server=1\n",
		filepath.Join("tls", "server", "cert.pem"): "cert\n",
		filepath.Join("tls", "server", "key.pem"):  "key\n",
		filepath.Join(".state", "live", "config-client", "xray.json"): "{}\n",
		filepath.Join(".state", "live", "config-server", "xray.json"): "{}\n",
		"audit.log": "audit\n",
		"random.tmp": "tmp\n",
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
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s to be missing", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertFileContains(t *testing.T, path string, needle string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), needle) {
		t.Fatalf("expected %s to contain %q, got: %s", path, needle, string(data))
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

