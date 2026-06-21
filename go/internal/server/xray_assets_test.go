package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCompileDesiredWritesXrayAssetsRuntimeMeta(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "xp2p-server.toml")
	if err := os.WriteFile(configPath, []byte(`
[xray_assets]
stale_after = "72h"

[[xray_assets.files]]
name = "geosite.dat"
url = "https://example.test/geosite.dat"
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
	if meta.XrayAssets.Files[0].Name != "geosite.dat" || meta.XrayAssets.Files[0].URL == "" {
		t.Fatalf("unexpected xray asset file metadata: %+v", meta.XrayAssets.Files[0])
	}
}

func TestCompileDesiredWritesControlRuntimeMeta(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "xp2p-server.toml")
	certPath, keyPath := createTestCertificateFiles(t, dir, "edge.example")
	if err := os.WriteFile(configPath, []byte(`
[server]
host = "edge.example"
port = "62022"
trojan_port = "58443"
certificate = "`+filepath.ToSlash(certPath)+`"
key = "`+filepath.ToSlash(keyPath)+`"

[[server.trojan_users]]
email = "alice"
password = "secret"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, err := compileDesired(configPath, filepath.Join(dir, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts.Extra) != 0 {
		t.Fatalf("unexpected extra control artifacts: %+v", artifacts.Extra)
	}
	var meta runtimeMeta
	if err := json.Unmarshal(artifacts.MetaJSON, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Control.Endpoint.Port != 62022 || meta.Control.Endpoint.Scheme != "https" {
		t.Fatalf("unexpected control endpoint: %+v", meta.Control.Endpoint)
	}
	if meta.Control.Subscription.Generation == "" {
		t.Fatalf("subscription generation missing: %+v", meta.Control.Subscription)
	}
	if meta.Control.Subscription.Protocol != "trojan" || meta.Control.Subscription.Port != 58443 {
		t.Fatalf("unexpected subscription: %+v", meta.Control.Subscription)
	}
	if meta.Control.Subscription.TLS.CertificatePath != "" || meta.Control.Subscription.TLS.SelfSigned {
		t.Fatalf("subscription leaked server-only TLS metadata: %+v", meta.Control.Subscription.TLS)
	}
	if meta.Control.TLS.CertificatePath == "" || !meta.Control.TLS.SelfSigned {
		t.Fatalf("runtime TLS metadata must keep server-only fields: %+v", meta.Control.TLS)
	}
	if len(meta.Control.AuthUsers) != 1 || meta.Control.AuthUsers[0].Label != "alice" {
		t.Fatalf("unexpected auth users: %+v", meta.Control.AuthUsers)
	}
}
