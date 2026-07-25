package root

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

type stage4ArchiveFixture struct {
	root        string
	configName  string
	metadata    []byte
	metadataRel string
}

func archiveStage4Contract(role, operation string) stage4Contract {
	return stage4Contract{
		success: func(t *testing.T, path string) {
			fixture := newStage4ArchiveFixture(t, role)
			switch operation {
			case "export":
				output := filepath.Join(fixture.root, role+"-\u96ea.zip")
				assertStage4ArchiveSuccess(t, path, []string{
					role, "export", "--config-root", fixture.root, "--output", output,
				}, output)
				assertStage4ArchiveMetadata(t, output, fixture.metadata)
			case "debug":
				output := filepath.Join(fixture.root, role+"-debug.zip")
				assertStage4ArchiveSuccess(t, path, []string{
					role, "debug", "bundle", "--output", output,
				}, output)
				assertStage4ArchiveMetadata(t, output, fixture.metadata)
			case "import":
				input := filepath.Join(fixture.root, role+"-source.zip")
				assertStage4ArchiveSuccess(t, "xp2p "+role+" export", []string{
					role, "export", "--config-root", fixture.root, "--output", input,
				}, input)
				assertStage4ArchiveMetadata(t, input, fixture.metadata)
				target := filepath.Join(fixture.root, "imported")
				execution := executeContractCase([]string{
					role, "import", "--config-root", target, "--input", input,
				}, false)
				result := assertStage4Success(t, path, execution)
				if result["status"] != "completed" || result["path"] != input {
					t.Fatalf("unexpected import result: %#v", result)
				}
				if _, err := os.Stat(filepath.Join(target, fixture.configName)); err != nil {
					t.Fatalf("imported artifact is absent: %v", err)
				}
				importedMetadata, err := os.ReadFile(filepath.Join(target, fixture.metadataRel))
				if err != nil || !bytes.Equal(importedMetadata, fixture.metadata) {
					t.Fatalf("imported metadata changed: content=%q err=%v", importedMetadata, err)
				}
			}
		},
		failure: func(t *testing.T, path string) {
			fixture := newStage4ArchiveFixture(t, role)
			secret := "stage4-archive-secret"
			switch operation {
			case "export":
				execution := executeContractCase([]string{
					role, "export",
					"--config-root", filepath.Join(fixture.root, "missing"),
					"--output", filepath.Join(fixture.root, "failed.zip"),
				}, false)
				assertStage4Failure(t, path, execution, secret)
			case "debug":
				execution := executeContractCase([]string{
					role, "debug", "bundle",
					"--output", filepath.Join(fixture.root, "unsupported.rar"),
				}, false)
				assertStage4Failure(t, path, execution, secret)
			case "import":
				target := filepath.Join(fixture.root, "existing")
				if err := os.MkdirAll(target, 0o755); err != nil {
					t.Fatal(err)
				}
				targetPath := filepath.Join(target, fixture.configName)
				baseline := []byte("baseline = \"unchanged\"\n")
				if err := os.WriteFile(targetPath, baseline, 0o600); err != nil {
					t.Fatal(err)
				}
				input := filepath.Join(fixture.root, "partial.tar.gz")
				writePartiallyExtractableArchive(t, input, fixture.configName, secret)
				execution := executeContractCase([]string{
					role, "import", "--config-root", target, "--input", input,
				}, false)
				assertStage4Failure(t, path, execution, secret)
				after, err := os.ReadFile(targetPath)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(after, baseline) {
					t.Fatalf("failed import changed target state: got=%q want=%q", after, baseline)
				}
			}
		},
		human: func(t *testing.T, path string) {
			fixture := newStage4ArchiveFixture(t, role)
			switch operation {
			case "export":
				output := filepath.Join(fixture.root, role+"-human.zip")
				stdout, stderr, err := executeHumanContractCase([]string{
					role, "export", "--config-root", fixture.root, "--output", output,
				})
				assertStage4Human(t, path, stdout, stderr, err, "archive created", output)
			case "debug":
				output := filepath.Join(fixture.root, role+"-human-debug.zip")
				stdout, stderr, err := executeHumanContractCase([]string{
					role, "debug", "bundle", "--output", output,
				})
				assertStage4Human(t, path, stdout, stderr, err, "archive created", output)
			case "import":
				input := filepath.Join(fixture.root, role+"-human-source.zip")
				assertStage4ArchiveSuccess(t, "xp2p "+role+" export", []string{
					role, "export", "--config-root", fixture.root, "--output", input,
				}, input)
				stdout, stderr, err := executeHumanContractCase([]string{
					role, "import", "--config-root", filepath.Join(fixture.root, "human-import"),
					"--input", input,
				})
				assertStage4Human(t, path, stdout, stderr, err, "archive applied", "verify service status")
			}
		},
	}
}

func newStage4ArchiveFixture(t *testing.T, role string) stage4ArchiveFixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	name := layout.ClientConfigFileName
	if role == "server" {
		name = layout.ServerConfigFileName
	}
	content := []byte(
		"version = \"0.2.9\"\ncredential = \"stage4-archive-secret\"\n" +
			"metadata = \"unicode-\u96ea\\n\\t\\u0001\"\n",
	)
	if err := os.WriteFile(filepath.Join(root, name), content, 0o600); err != nil {
		t.Fatal(err)
	}
	configDir := layout.ClientConfigDir
	if role == "server" {
		configDir = layout.ServerConfigDir
	}
	metadata := []byte("{\"label\":\"unicode-\u96ea\\n\\t\\u0001\"}")
	metadataDir := filepath.Join(root, configDir)
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metadataRel := filepath.Join(configDir, "metadata-\u96ea.json")
	if err := os.WriteFile(filepath.Join(root, metadataRel), metadata, 0o600); err != nil {
		t.Fatal(err)
	}
	return stage4ArchiveFixture{
		root: root, configName: name, metadata: metadata, metadataRel: metadataRel,
	}
}

func writePartiallyExtractableArchive(t *testing.T, path, configName, secret string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gz)
	content := []byte("credential = \"" + secret + "\"\n")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: configName, Mode: 0o600, Size: int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: filepath.ToSlash(filepath.Join(layout.StateDirName, "truncated.json")),
		Mode: 0o600, Size: 100,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte("{")); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
