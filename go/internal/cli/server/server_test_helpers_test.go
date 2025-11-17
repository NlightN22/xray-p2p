package servercmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func serverCfg(installDir, configDir, host string) config.Config {
	return config.Config{Server: config.ServerConfig{InstallDir: installDir, ConfigDir: configDir, Host: host}}
}

func requireEqual[T comparable](t *testing.T, got, want T, label string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s mismatch: got %#v want %#v", label, got, want)
	}
}

func stubServerInstall(fn func(context.Context, server.InstallOptions) error) func() {
	prev := serverInstallFunc
	if fn == nil {
		fn = func(context.Context, server.InstallOptions) error { return nil }
	}
	serverInstallFunc = fn
	return func() { serverInstallFunc = prev }
}

func stubServerRemove(fn func(context.Context, server.RemoveOptions) error) func() {
	prev := serverRemoveFunc
	if fn == nil {
		fn = func(context.Context, server.RemoveOptions) error { return nil }
	}
	serverRemoveFunc = fn
	return func() { serverRemoveFunc = prev }
}

func stubServerRun(fn func(context.Context, server.RunOptions) error) func() {
	prev := serverRunFunc
	if fn == nil {
		fn = func(context.Context, server.RunOptions) error { return nil }
	}
	serverRunFunc = fn
	return func() { serverRunFunc = prev }
}

func stubDetectPublicHost(value string, err error) func() {
	prev := detectPublicHostFunc
	detectPublicHostFunc = func(context.Context) (string, error) { return value, err }
	return func() { detectPublicHostFunc = prev }
}

func stubServerSetCertificate(fn func(context.Context, server.CertificateOptions) error) func() {
	prev := serverSetCertFunc
	if fn == nil {
		fn = func(context.Context, server.CertificateOptions) error { return nil }
	}
	serverSetCertFunc = fn
	return func() { serverSetCertFunc = prev }
}

func stubPromptYesNo(answer bool, err error) func() {
	prev := promptYesNoFunc
	promptYesNoFunc = func(string) (bool, error) { return answer, err }
	return func() { promptYesNoFunc = prev }
}

func stubServerUserAdd(fn func(context.Context, server.AddUserOptions) error) func() {
	prev := serverUserAddFunc
	if fn == nil {
		fn = func(context.Context, server.AddUserOptions) error { return nil }
	}
	serverUserAddFunc = fn
	return func() { serverUserAddFunc = prev }
}

func stubServerUserLink(fn func(context.Context, server.UserLinkOptions) (server.UserLink, error)) func() {
	prev := serverUserLinkFunc
	if fn == nil {
		fn = func(context.Context, server.UserLinkOptions) (server.UserLink, error) { return server.UserLink{}, nil }
	}
	serverUserLinkFunc = fn
	return func() { serverUserLinkFunc = prev }
}

func stubServerUserList(fn func(context.Context, server.ListUsersOptions) ([]server.UserLink, error)) func() {
	prev := serverUserListFunc
	if fn == nil {
		fn = func(context.Context, server.ListUsersOptions) ([]server.UserLink, error) {
			return []server.UserLink{}, nil
		}
	}
	serverUserListFunc = fn
	return func() { serverUserListFunc = prev }
}

func stubServerReverseList(fn func(server.ReverseListOptions) ([]server.ReverseRecord, error)) func() {
	prev := serverReverseListFunc
	if fn == nil {
		fn = func(server.ReverseListOptions) ([]server.ReverseRecord, error) { return nil, nil }
	}
	serverReverseListFunc = fn
	return func() { serverReverseListFunc = prev }
}

func stubServerRedirectAdd(fn func(server.RedirectAddOptions) error) func() {
	prev := serverRedirectAddFunc
	if fn == nil {
		fn = func(server.RedirectAddOptions) error { return nil }
	}
	serverRedirectAddFunc = fn
	return func() { serverRedirectAddFunc = prev }
}

func stubServerRedirectPromptReader(reader io.Reader) func() {
	prev := serverRedirectPromptReader
	serverRedirectPromptReader = func() io.Reader {
		if reader != nil {
			return reader
		}
		return os.Stdin
	}
	return func() { serverRedirectPromptReader = prev }
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

func prepareInstallation(t *testing.T, installDir, configDirName string) {
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
