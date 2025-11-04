package clientcmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func clientCfg(installDir, configDir string) config.Config {
	return config.Config{
		Client: config.ClientConfig{
			InstallDir: installDir,
			ConfigDir:  configDir,
		},
	}
}

func stubClientInstall(fn func(context.Context, client.InstallOptions) error) func() {
	prev := clientInstallFunc
	if fn == nil {
		fn = func(context.Context, client.InstallOptions) error { return nil }
	}
	clientInstallFunc = fn
	return func() { clientInstallFunc = prev }
}

func stubClientRemove(fn func(context.Context, client.RemoveOptions) error) func() {
	prev := clientRemoveFunc
	if fn == nil {
		fn = func(context.Context, client.RemoveOptions) error { return nil }
	}
	clientRemoveFunc = fn
	return func() { clientRemoveFunc = prev }
}

func stubClientRun(fn func(context.Context, client.RunOptions) error) func() {
	prev := clientRunFunc
	if fn == nil {
		fn = func(context.Context, client.RunOptions) error { return nil }
	}
	clientRunFunc = fn
	return func() { clientRunFunc = prev }
}

func prepareClientInstallation(t *testing.T, installDir, configDirName string) {
	t.Helper()

	binDir := filepath.Join(installDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", binDir, err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "xray.exe"), []byte{}, 0o755); err != nil {
		t.Fatalf("write xray.exe: %v", err)
	}

	configDir := filepath.Join(installDir, configDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", configDir, err)
	}

	for _, name := range []string{"inbounds.json", "logs.json", "outbounds.json", "routing.json"} {
		path := filepath.Join(configDir, name)
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

func requireEqual[T comparable](t *testing.T, got, want T, label string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s mismatch: got %v want %v", label, got, want)
	}
}
