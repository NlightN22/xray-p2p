package clientcmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
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

func stubClientRemoveEndpoint(fn func(context.Context, client.RemoveEndpointOptions) error) func() {
	prev := clientRemoveEndpointFunc
	if fn == nil {
		fn = func(context.Context, client.RemoveEndpointOptions) error { return nil }
	}
	clientRemoveEndpointFunc = fn
	return func() { clientRemoveEndpointFunc = prev }
}

func stubClientRun(fn func(context.Context, client.RunOptions) error) func() {
	prev := clientRunFunc
	if fn == nil {
		fn = func(context.Context, client.RunOptions) error { return nil }
	}
	clientRunFunc = fn
	return func() { clientRunFunc = prev }
}

func stubClientList(fn func(client.ListOptions) ([]client.EndpointRecord, error)) func() {
	prev := clientListFunc
	if fn == nil {
		fn = func(client.ListOptions) ([]client.EndpointRecord, error) { return nil, nil }
	}
	clientListFunc = fn
	return func() { clientListFunc = prev }
}

func stubClientReverseList(fn func(client.ReverseListOptions) ([]client.ReverseRecord, error)) func() {
	prev := clientReverseListFunc
	if fn == nil {
		fn = func(client.ReverseListOptions) ([]client.ReverseRecord, error) { return nil, nil }
	}
	clientReverseListFunc = fn
	return func() { clientReverseListFunc = prev }
}

func stubClientRedirectAdd(fn func(client.RedirectAddOptions) error) func() {
	prev := clientRedirectAddFunc
	if fn == nil {
		fn = func(client.RedirectAddOptions) error { return nil }
	}
	clientRedirectAddFunc = fn
	return func() { clientRedirectAddFunc = prev }
}

func stubClientRedirectPromptReader(reader io.Reader) func() {
	prev := clientRedirectPromptReader
	clientRedirectPromptReader = func() io.Reader {
		if reader != nil {
			return reader
		}
		return os.Stdin
	}
	return func() { clientRedirectPromptReader = prev }
}

func prepareClientInstallation(t *testing.T, installDir, configDirName string) {
	t.Helper()

	binDir := filepath.Join(installDir, layout.BinDirName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", binDir, err)
	}

	binaries := []string{"xray.exe"}
	if runtime.GOOS != "windows" {
		binaries = append(binaries, "xray")
	}
	for _, name := range binaries {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte{}, 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close stdout: %v", err)
	}
	os.Stdout = oldStdout
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}
