package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

	cfg := config.Config{}
	code := runServerInstall(context.Background(), cfg, nil)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
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
