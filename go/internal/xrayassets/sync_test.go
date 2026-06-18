package xrayassets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateName(t *testing.T) {
	valid := []string{"geoip.dat", "geosite.dat", "custom-name.dat"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Fatalf("ValidateName(%q) returned %v", name, err)
		}
	}
	invalid := []string{"", "../geoip.dat", "dir/geoip.dat", `dir\geoip.dat`, "/geoip.dat", "geoip.txt", "geo..ip.dat"}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Fatalf("ValidateName(%q) succeeded", name)
		}
	}
}

func TestSyncMissingWithoutURLFails(t *testing.T) {
	err := Sync(context.Background(), Config{Files: []File{{Name: "geosite.dat"}}}, Options{AssetDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "required asset geosite.dat is missing") {
		t.Fatalf("expected missing asset error, got %v", err)
	}
}

func TestSyncMissingWithURLDownloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("asset"))
	}))
	defer server.Close()

	dir := t.TempDir()
	err := Sync(context.Background(), Config{Files: []File{{Name: "geoip.dat", URL: server.URL}}}, Options{AssetDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "geoip.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "asset" {
		t.Fatalf("unexpected asset content %q", data)
	}
}

func TestSyncMissingDownloadFailureIsFatal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer server.Close()

	err := Sync(context.Background(), Config{Files: []File{{Name: "geoip.dat", URL: server.URL}}}, Options{AssetDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "download failed") {
		t.Fatalf("expected fatal download error, got %v", err)
	}
}

func TestSyncFreshExistingDoesNotDownload(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte("new"))
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "geoip.dat")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Sync(context.Background(), Config{Files: []File{{Name: "geoip.dat", URL: server.URL, StaleAfter: time.Hour}}}, Options{AssetDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("server was called for fresh asset")
	}
}

func TestSyncStaleExistingDownloadsReplacement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("new"))
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "geoip.dat")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	err := Sync(context.Background(), Config{Files: []File{{Name: "geoip.dat", URL: server.URL, StaleAfter: time.Hour}}}, Options{AssetDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("unexpected asset content %q", data)
	}
}

func TestSyncStaleExistingFailedRefreshKeepsFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "geoip.dat")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	err := Sync(context.Background(), Config{Files: []File{{Name: "geoip.dat", URL: server.URL, StaleAfter: time.Hour}}}, Options{AssetDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("stale failed refresh replaced content with %q", data)
	}
}

func TestRequiredFromXrayConfigDetectsAssets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xray.json")
	doc := `{"routing":{"rules":[{"domain":["geosite:private","ext:custom.dat:list"],"ip":["geoip:cn"]}]}}`
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := RequiredFromXrayConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, ",")
	for _, want := range []string{"custom.dat", "geoip.dat", "geosite.dat"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %s in %v", want, got)
		}
	}
}
