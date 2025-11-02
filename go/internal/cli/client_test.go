package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestRunClientInstallUsesCLIOverrides(t *testing.T) {
	restore := stubClientInstall(func(ctx context.Context, opts client.InstallOptions) error {
		if opts.InstallDir != `D:\xp2p-client` {
			t.Fatalf("unexpected install dir: %s", opts.InstallDir)
		}
		if opts.ConfigDir != "cfg-client" {
			t.Fatalf("unexpected config dir: %s", opts.ConfigDir)
		}
		if opts.ServerAddress != "example.org" {
			t.Fatalf("unexpected server address: %s", opts.ServerAddress)
		}
		if opts.ServerPort != "9443" {
			t.Fatalf("unexpected server port: %s", opts.ServerPort)
		}
		if opts.Password != "secret" {
			t.Fatalf("unexpected password: %s", opts.Password)
		}
		if opts.ServerName != "custom.name" {
			t.Fatalf("unexpected server name: %s", opts.ServerName)
		}
		if !opts.AllowInsecure {
			t.Fatalf("expected allow insecure to be true from CLI flag")
		}
		if !opts.Force {
			t.Fatalf("expected force overwrite")
		}
		return nil
	})
	defer restore()

	cfg := config.Config{
		Client: config.ClientConfig{
			InstallDir:    `C:\xp2p-client`,
			ConfigDir:     "config-client",
			ServerAddress: "default",
			ServerPort:    "8443",
			Password:      "default-password",
			ServerName:    "default.name",
			AllowInsecure: false,
		},
	}

	code := runClientInstall(
		context.Background(),
		cfg,
		[]string{
			"--path", `D:\xp2p-client`,
			"--config-dir", "cfg-client",
			"--server-address", "example.org",
			"--server-port", "9443",
			"--password", "secret",
			"--server-name", "custom.name",
			"--allow-insecure",
			"--force",
		},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunClientInstallPropagatesErrors(t *testing.T) {
	restore := stubClientInstall(func(ctx context.Context, opts client.InstallOptions) error {
		return errors.New("install failure")
	})
	defer restore()

	cfg := config.Config{
		Client: config.ClientConfig{
			InstallDir:    `C:\xp2p-client`,
			ConfigDir:     "config-client",
			ServerAddress: "host",
			ServerPort:    "8443",
			Password:      "secret",
		},
	}

	code := runClientInstall(context.Background(), cfg, nil)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestRunClientRemoveUsesDefaults(t *testing.T) {
	restoreInstall := stubClientInstall(nil)
	defer restoreInstall()
	restoreRemove := stubClientRemove(func(ctx context.Context, opts client.RemoveOptions) error {
		if opts.InstallDir != `C:\xp2p-client` {
			t.Fatalf("unexpected install dir: %s", opts.InstallDir)
		}
		if opts.KeepFiles {
			t.Fatalf("expected default keep-files false")
		}
		if opts.IgnoreMissing {
			t.Fatalf("expected default ignore-missing false")
		}
		return nil
	})
	defer restoreRemove()

	cfg := config.Config{
		Client: config.ClientConfig{
			InstallDir: `C:\xp2p-client`,
		},
	}

	code := runClientRemove(context.Background(), cfg, nil)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunClientRemoveFlags(t *testing.T) {
	restoreInstall := stubClientInstall(nil)
	defer restoreInstall()
	restoreRemove := stubClientRemove(func(ctx context.Context, opts client.RemoveOptions) error {
		if opts.InstallDir != `D:\xp2p-client` {
			t.Fatalf("unexpected install dir: %s", opts.InstallDir)
		}
		if !opts.KeepFiles {
			t.Fatalf("expected keep-files true")
		}
		if !opts.IgnoreMissing {
			t.Fatalf("expected ignore-missing true")
		}
		return nil
	})
	defer restoreRemove()

	cfg := config.Config{
		Client: config.ClientConfig{
			InstallDir: `C:\xp2p-client`,
		},
	}

	code := runClientRemove(
		context.Background(),
		cfg,
		[]string{
			"--path", `D:\xp2p-client`,
			"--keep-files",
			"--ignore-missing",
		},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunClientRunAutoInstall(t *testing.T) {
	var installCalled bool
	var runCalled bool

	restoreInstall := stubClientInstall(func(ctx context.Context, opts client.InstallOptions) error {
		installCalled = true
		if opts.InstallDir == "" {
			t.Fatalf("expected install dir provided")
		}
		if opts.ServerAddress != "example.org" {
			t.Fatalf("unexpected server address: %s", opts.ServerAddress)
		}
		if opts.Password != "secret" {
			t.Fatalf("unexpected password: %s", opts.Password)
		}
		return nil
	})
	defer restoreInstall()
	restoreRun := stubClientRun(func(ctx context.Context, opts client.RunOptions) error {
		runCalled = true
		return nil
	})
	defer restoreRun()

	installDir := filepath.Join(t.TempDir(), "client")

	cfg := config.Config{
		Client: config.ClientConfig{
			InstallDir:    installDir,
			ConfigDir:     client.DefaultClientConfigDir,
			ServerAddress: "example.org",
			ServerPort:    "9443",
			Password:      "secret",
			ServerName:    "sni.example.org",
			AllowInsecure: false,
		},
	}

	code := runClientRun(context.Background(), cfg, []string{"--auto-install"})
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

func TestRunClientRunWithLogFile(t *testing.T) {
	restoreInstall := stubClientInstall(nil)
	defer restoreInstall()
	restoreRun := stubClientRun(func(ctx context.Context, opts client.RunOptions) error {
		if opts.ErrorLogPath != `logs\xray.err` {
			t.Fatalf("unexpected error log path: %s", opts.ErrorLogPath)
		}
		return nil
	})
	defer restoreRun()

	installDir := filepath.Join(t.TempDir(), "client")
	prepareClientInstallation(t, installDir, client.DefaultClientConfigDir)

	cfg := config.Config{
		Client: config.ClientConfig{
			InstallDir: installDir,
			ConfigDir:  client.DefaultClientConfigDir,
		},
	}

	code := runClientRun(context.Background(), cfg, []string{"--xray-log-file", `logs\xray.err`})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunClientRunQuietMissing(t *testing.T) {
	restoreInstall := stubClientInstall(func(ctx context.Context, opts client.InstallOptions) error {
		t.Fatalf("install should not be called when quiet")
		return nil
	})
	defer restoreInstall()
	restoreRun := stubClientRun(func(ctx context.Context, opts client.RunOptions) error {
		t.Fatalf("run should not be invoked when installation missing")
		return nil
	})
	defer restoreRun()

	cfg := config.Config{
		Client: config.ClientConfig{
			InstallDir: filepath.Join(t.TempDir(), "client"),
			ConfigDir:  client.DefaultClientConfigDir,
		},
	}

	code := runClientRun(context.Background(), cfg, []string{"--quiet"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code when installation missing in quiet mode")
	}
}

func stubClientInstall(fn func(context.Context, client.InstallOptions) error) func() {
	prev := clientInstallFunc
	if fn != nil {
		clientInstallFunc = fn
	} else {
		clientInstallFunc = func(context.Context, client.InstallOptions) error { return nil }
	}
	return func() {
		clientInstallFunc = prev
	}
}

func stubClientRemove(fn func(context.Context, client.RemoveOptions) error) func() {
	prev := clientRemoveFunc
	if fn != nil {
		clientRemoveFunc = fn
	} else {
		clientRemoveFunc = func(context.Context, client.RemoveOptions) error { return nil }
	}
	return func() {
		clientRemoveFunc = prev
	}
}

func stubClientRun(fn func(context.Context, client.RunOptions) error) func() {
	prev := clientRunFunc
	if fn != nil {
		clientRunFunc = fn
	} else {
		clientRunFunc = func(context.Context, client.RunOptions) error { return nil }
	}
	return func() {
		clientRunFunc = prev
	}
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

	files := []string{"inbounds.json", "logs.json", "outbounds.json", "routing.json"}
	for _, name := range files {
		path := filepath.Join(configDir, name)
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}
