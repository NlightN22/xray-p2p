package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCompileDesiredWritesXrayAssetsRuntimeMeta(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "xp2p-client.toml")
	if err := os.WriteFile(configPath, []byte(`
[xray_assets]
stale_after = "72h"

[[xray_assets.files]]
name = "geoip.dat"
url = "https://example.test/geoip.dat"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, err := compileDesired(configPath, filepath.Join(dir, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	var meta runtimeMeta
	if err := json.Unmarshal(artifacts.MetaJSON, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.XrayAssets.StaleAfter != "72h" || len(meta.XrayAssets.Files) != 1 {
		t.Fatalf("unexpected xray assets metadata: %+v", meta.XrayAssets)
	}
	if meta.XrayAssets.Files[0].Name != "geoip.dat" || meta.XrayAssets.Files[0].URL == "" {
		t.Fatalf("unexpected xray asset file metadata: %+v", meta.XrayAssets.Files[0])
	}
}
