package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerInstallUsesCLIOverrides(t *testing.T) {
	restore := stubServerInstall(func(ctx context.Context, opts server.InstallOptions) error {
		if opts.InstallDir != `D:\xp2p` {
			t.Fatalf("unexpected install dir: %s", opts.InstallDir)
		}
		if opts.ConfigDir != "cfg-custom" {
			t.Fatalf("unexpected config dir: %s", opts.ConfigDir)
		}
		if opts.Port != "65000" {
			t.Fatalf("unexpected port: %s", opts.Port)
		}
		if !opts.Force {
			t.Fatalf("expected force overwrite")
		}
		if opts.CertificateFile != `C:\certs\server.pem` {
			t.Fatalf("unexpected certificate: %s", opts.CertificateFile)
		}
		if opts.KeyFile != `C:\certs\server.key` {
			t.Fatalf("unexpected key: %s", opts.KeyFile)
		}
		if opts.Host != "custom.example.test" {
			t.Fatalf("unexpected host: %s", opts.Host)
		}
		return nil
	})
	defer restore()

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir:      `C:\programdata\xp2p`,
			Port:            "62022",
			ConfigDir:       "config-server",
			CertificateFile: "",
			KeyFile:         "",
			Host:            "",
		},
	}

	code := runServerInstall(
		context.Background(),
		cfg,
		[]string{
			"--path", `D:\xp2p`,
			"--config-dir", "cfg-custom",
			"--port", "65000",
			"--cert", `C:\certs\server.pem`,
			"--key", `C:\certs\server.key`,
			"--host", "custom.example.test",
			"--force",
		},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunServerInstallPropagatesErrors(t *testing.T) {
	restore := stubServerInstall(func(ctx context.Context, opts server.InstallOptions) error {
		return errors.New("install failure")
	})
	defer restore()
	restoreDetect := stubDetectPublicHost("198.51.100.10", nil)
	defer restoreDetect()

	cfg := config.Config{
		Server: config.ServerConfig{
			Host: "",
		},
	}
	code := runServerInstall(context.Background(), cfg, nil)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestRunServerInstallFailsWhenHostDetectionFails(t *testing.T) {
	restoreInstall := stubServerInstall(func(ctx context.Context, opts server.InstallOptions) error {
		t.Fatalf("install should not be called when host detection fails")
		return nil
	})
	defer restoreInstall()
	restoreDetect := stubDetectPublicHost("", errors.New("no host"))
	defer restoreDetect()

	cfg := config.Config{
		Server: config.ServerConfig{},
	}

	code := runServerInstall(context.Background(), cfg, nil)
	if code != 1 {
		t.Fatalf("expected exit code 1 when host detection fails, got %d", code)
	}
}

func TestRunServerCertSetUsesFlags(t *testing.T) {
	restoreCert := stubServerSetCertificate(func(ctx context.Context, opts server.CertificateOptions) error {
		if opts.InstallDir != `D:\xp2p` {
			t.Fatalf("unexpected install dir: %s", opts.InstallDir)
		}
		if opts.ConfigDir != "cfg-custom" {
			t.Fatalf("unexpected config dir: %s", opts.ConfigDir)
		}
		if opts.CertificateFile != `C:\certs\server.pem` {
			t.Fatalf("unexpected certificate file: %s", opts.CertificateFile)
		}
		if opts.KeyFile != `C:\certs\server.key` {
			t.Fatalf("unexpected key file: %s", opts.KeyFile)
		}
		if opts.Host != "cert.example.test" {
			t.Fatalf("unexpected host: %s", opts.Host)
		}
		if !opts.Force {
			t.Fatalf("expected force to propagate")
		}
		return nil
	})
	defer restoreCert()

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\xp2p`,
			ConfigDir:  server.DefaultServerConfigDir,
			Host:       "",
		},
	}

	code := runServerCertSet(
		context.Background(),
		cfg,
		[]string{
			"--path", `D:\xp2p`,
			"--config-dir", "cfg-custom",
			"--cert", `C:\certs\server.pem`,
			"--key", `C:\certs\server.key`,
			"--host", "cert.example.test",
			"--force",
		},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunServerCertSetDetectsHostWhenMissing(t *testing.T) {
	restoreCert := stubServerSetCertificate(func(ctx context.Context, opts server.CertificateOptions) error {
		if opts.Host != "198.51.100.20" {
			t.Fatalf("unexpected detected host: %s", opts.Host)
		}
		return nil
	})
	defer restoreCert()
	restoreDetect := stubDetectPublicHost("198.51.100.20", nil)
	defer restoreDetect()

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\xp2p`,
			ConfigDir:  server.DefaultServerConfigDir,
			Host:       "",
		},
	}

	code := runServerCertSet(
		context.Background(),
		cfg,
		[]string{
			"--path", `C:\xp2p`,
		},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunServerCertSetPromptsWhenCertificateExists(t *testing.T) {
	callCount := 0
	restoreCert := stubServerSetCertificate(func(ctx context.Context, opts server.CertificateOptions) error {
		callCount++
		if callCount == 1 {
			if opts.Force {
				t.Fatalf("expected first attempt without force")
			}
			return server.ErrCertificateConfigured
		}
		if !opts.Force {
			t.Fatalf("expected second attempt with force")
		}
		return nil
	})
	defer restoreCert()
	restorePrompt := stubPromptYesNo(true, nil)
	defer restorePrompt()

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\xp2p`,
			ConfigDir:  server.DefaultServerConfigDir,
			Host:       "configured.example.test",
		},
	}

	code := runServerCertSet(
		context.Background(),
		cfg,
		[]string{
			"--path", `C:\xp2p`,
		},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 after confirmation, got %d", code)
	}
	if callCount != 2 {
		t.Fatalf("expected SetCertificate to be called twice, got %d", callCount)
	}
}

func TestRunServerRemoveUsesDefaults(t *testing.T) {
	restoreInstall := stubServerInstall(nil)
	defer restoreInstall()
	restoreRemove := stubServerRemove(func(ctx context.Context, opts server.RemoveOptions) error {
		if opts.InstallDir != `C:\xp2p` {
			t.Fatalf("unexpected install dir: %s", opts.InstallDir)
		}
		if opts.KeepFiles {
			t.Fatalf("expected files to be removed by default")
		}
		if opts.IgnoreMissing {
			t.Fatalf("expected ignoreMissing to default to false")
		}
		return nil
	})
	defer restoreRemove()

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\xp2p`,
		},
	}

	code := runServerRemove(context.Background(), cfg, nil)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunServerRemoveFlags(t *testing.T) {
	restoreInstall := stubServerInstall(nil)
	defer restoreInstall()
	restoreRemove := stubServerRemove(func(ctx context.Context, opts server.RemoveOptions) error {
		if opts.InstallDir != `D:\xp2p` {
			t.Fatalf("unexpected install dir: %s", opts.InstallDir)
		}
		if !opts.KeepFiles {
			t.Fatalf("expected keep-files to be true")
		}
		if !opts.IgnoreMissing {
			t.Fatalf("expected ignore-missing to be true")
		}
		return nil
	})
	defer restoreRemove()

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\xp2p`,
		},
	}

	code := runServerRemove(
		context.Background(),
		cfg,
		[]string{
			"--path", `D:\xp2p`,
			"--keep-files",
			"--ignore-missing",
		},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunServerRunUsesExistingInstall(t *testing.T) {
	restoreInstall := stubServerInstall(nil)
	defer restoreInstall()

	called := false
	restoreRun := stubServerRun(func(ctx context.Context, opts server.RunOptions) error {
		called = true
		if opts.InstallDir == "" {
			t.Fatalf("expected install dir to be set")
		}
		if opts.ConfigDir != server.DefaultServerConfigDir {
			t.Fatalf("unexpected config dir: %s", opts.ConfigDir)
		}
		if opts.ErrorLogPath != "" {
			t.Fatalf("expected empty error log path, got %s", opts.ErrorLogPath)
		}
		return nil
	})
	defer restoreRun()

	installDir := filepath.Join(t.TempDir(), "srv")
	prepareInstallation(t, installDir, server.DefaultServerConfigDir)

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: installDir,
			ConfigDir:  server.DefaultServerConfigDir,
		},
	}

	code := runServerRun(context.Background(), cfg, nil)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !called {
		t.Fatalf("expected serverRunFunc to be called")
	}
}

func TestRunServerRunAutoInstall(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "srv")

	installCalled := false
	restoreInstall := stubServerInstall(func(ctx context.Context, opts server.InstallOptions) error {
		installCalled = true
		if opts.InstallDir != installDir {
			t.Fatalf("unexpected install dir: %s", opts.InstallDir)
		}
		if opts.ConfigDir != "config-server" {
			t.Fatalf("unexpected config dir: %s", opts.ConfigDir)
		}
		prepareInstallation(t, installDir, opts.ConfigDir)
		return nil
	})
	defer restoreInstall()

	runCalled := false
	restoreRun := stubServerRun(func(ctx context.Context, opts server.RunOptions) error {
		runCalled = true
		if opts.ErrorLogPath != "" {
			t.Fatalf("expected empty error log path, got %s", opts.ErrorLogPath)
		}
		return nil
	})
	defer restoreRun()
	restoreDetect := stubDetectPublicHost("198.51.100.20", nil)
	defer restoreDetect()

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: installDir,
			ConfigDir:  "config-server",
		},
	}

	code := runServerRun(context.Background(), cfg, []string{"--auto-install"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !installCalled {
		t.Fatalf("expected install to be triggered")
	}
	if !runCalled {
		t.Fatalf("expected run to be invoked")
	}
}

func TestRunServerRunWithLogFile(t *testing.T) {
	restoreInstall := stubServerInstall(nil)
	defer restoreInstall()
	restoreRun := stubServerRun(func(ctx context.Context, opts server.RunOptions) error {
		if opts.ErrorLogPath != `logs\xray.err` {
			t.Fatalf("unexpected error log path: %s", opts.ErrorLogPath)
		}
		return nil
	})
	defer restoreRun()

	installDir := filepath.Join(t.TempDir(), "srv")
	prepareInstallation(t, installDir, server.DefaultServerConfigDir)

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: installDir,
			ConfigDir:  server.DefaultServerConfigDir,
		},
	}

	code := runServerRun(context.Background(), cfg, []string{"--xray-log-file", `logs\xray.err`})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunServerRunQuietMissing(t *testing.T) {
	restoreInstall := stubServerInstall(func(ctx context.Context, opts server.InstallOptions) error {
		t.Fatalf("install should not be called in quiet mode without auto-install")
		return nil
	})
	defer restoreInstall()

	restoreRun := stubServerRun(func(ctx context.Context, opts server.RunOptions) error {
		t.Fatalf("run should not be invoked when installation missing")
		return nil
	})
	defer restoreRun()

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: filepath.Join(t.TempDir(), "srv"),
			ConfigDir:  server.DefaultServerConfigDir,
		},
	}

	code := runServerRun(context.Background(), cfg, []string{"--quiet"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code when installation missing in quiet mode")
	}
}

func TestRunServerUserAddPrintsLink(t *testing.T) {
	restoreAdd := stubServerUserAdd(func(context.Context, server.AddUserOptions) error { return nil })
	defer restoreAdd()
	restoreLink := stubServerUserLink(func(context.Context, server.UserLinkOptions) (server.UserLink, error) {
		return server.UserLink{
			UserID:   "alpha",
			Password: "secret",
			Link:     "trojan://secret@example.test:62022?allowInsecure=1&security=tls&sni=example.test#alpha",
		}, nil
	})
	defer restoreLink()

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\xp2p`,
			ConfigDir:  "config-server",
			Host:       "example.test",
		},
	}

	output := captureStdout(t, func() {
		code := runServerUserAdd(context.Background(), cfg, []string{
			"--path", `C:\xp2p`,
			"--config-dir", "config-server",
			"--id", "alpha",
			"--password", "secret",
		})
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(output, "trojan://secret@example.test:62022") {
		t.Fatalf("expected trojan link in output, got %q", output)
	}
}

func TestRunServerUserListPrintsLinks(t *testing.T) {
	restoreList := stubServerUserList(func(context.Context, server.ListUsersOptions) ([]server.UserLink, error) {
		return []server.UserLink{
			{UserID: "alpha", Link: "trojan://a"},
			{UserID: "", Link: "trojan://b"},
		}, nil
	})
	defer restoreList()

	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\xp2p`,
			ConfigDir:  "config-server",
		},
	}

	output := captureStdout(t, func() {
		code := runServerUser(context.Background(), cfg, []string{"list"})
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	})
	if !strings.Contains(output, "alpha: trojan://a") {
		t.Fatalf("expected entry for alpha, got %q", output)
	}
	if !strings.Contains(output, "(unnamed): trojan://b") {
		t.Fatalf("expected unnamed entry, got %q", output)
	}
}

func stubServerInstall(fn func(context.Context, server.InstallOptions) error) func() {
	prev := serverInstallFunc
	if fn != nil {
		serverInstallFunc = fn
	} else {
		serverInstallFunc = func(context.Context, server.InstallOptions) error { return nil }
	}
	return func() {
		serverInstallFunc = prev
	}
}

func stubServerRemove(fn func(context.Context, server.RemoveOptions) error) func() {
	prev := serverRemoveFunc
	if fn != nil {
		serverRemoveFunc = fn
	} else {
		serverRemoveFunc = func(context.Context, server.RemoveOptions) error { return nil }
	}
	return func() {
		serverRemoveFunc = prev
	}
}

func stubServerRun(fn func(context.Context, server.RunOptions) error) func() {
	prev := serverRunFunc
	if fn != nil {
		serverRunFunc = fn
	} else {
		serverRunFunc = func(context.Context, server.RunOptions) error { return nil }
	}
	return func() {
		serverRunFunc = prev
	}
}

func stubDetectPublicHost(value string, err error) func() {
	prev := detectPublicHostFunc
	detectPublicHostFunc = func(context.Context) (string, error) {
		return value, err
	}
	return func() {
		detectPublicHostFunc = prev
	}
}

func stubServerSetCertificate(fn func(context.Context, server.CertificateOptions) error) func() {
	prev := serverSetCertFunc
	if fn != nil {
		serverSetCertFunc = fn
	} else {
		serverSetCertFunc = func(context.Context, server.CertificateOptions) error { return nil }
	}
	return func() {
		serverSetCertFunc = prev
	}
}

func stubPromptYesNo(answer bool, err error) func() {
	prev := promptYesNoFunc
	promptYesNoFunc = func(string) (bool, error) {
		return answer, err
	}
	return func() {
		promptYesNoFunc = prev
	}
}

func stubServerUserAdd(fn func(context.Context, server.AddUserOptions) error) func() {
	prev := serverUserAddFunc
	if fn != nil {
		serverUserAddFunc = fn
	} else {
		serverUserAddFunc = func(context.Context, server.AddUserOptions) error { return nil }
	}
	return func() {
		serverUserAddFunc = prev
	}
}

func stubServerUserLink(fn func(context.Context, server.UserLinkOptions) (server.UserLink, error)) func() {
	prev := serverUserLinkFunc
	if fn != nil {
		serverUserLinkFunc = fn
	} else {
		serverUserLinkFunc = func(context.Context, server.UserLinkOptions) (server.UserLink, error) {
			return server.UserLink{}, nil
		}
	}
	return func() {
		serverUserLinkFunc = prev
	}
}

func stubServerUserList(fn func(context.Context, server.ListUsersOptions) ([]server.UserLink, error)) func() {
	prev := serverUserListFunc
	if fn != nil {
		serverUserListFunc = fn
	} else {
		serverUserListFunc = func(context.Context, server.ListUsersOptions) ([]server.UserLink, error) {
			return []server.UserLink{}, nil
		}
	}
	return func() {
		serverUserListFunc = prev
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
		t.Fatalf("close stdout writer: %v", err)
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

	files := []string{"inbounds.json", "logs.json", "outbounds.json", "routing.json"}
	for _, name := range files {
		path := filepath.Join(configDir, name)
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}
