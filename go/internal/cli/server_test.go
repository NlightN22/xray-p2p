package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerInstallUsesCLIOverrides(t *testing.T) {
	restore := stubServerInstall(func(ctx context.Context, opts server.InstallOptions) error {
		if opts.InstallDir != `D:\xp2p` {
			t.Fatalf("unexpected install dir: %s", opts.InstallDir)
		}
		if opts.Port != "65000" {
			t.Fatalf("unexpected port: %s", opts.Port)
		}
		if opts.Mode != "manual" {
			t.Fatalf("unexpected mode: %s", opts.Mode)
		}
		if !opts.Force {
			t.Fatalf("expected force overwrite")
		}
		if opts.StartService {
			t.Fatalf("expected start flag to be false")
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
			Mode:            "auto",
			CertificateFile: "",
			KeyFile:         "",
		},
	}

	code := runServerInstall(
		context.Background(),
		cfg,
		[]string{
			"--path", `D:\xp2p`,
			"--port", "65000",
			"--mode", "manual",
			"--cert", `C:\certs\server.pem`,
			"--key", `C:\certs\server.key`,
			"--force",
			"--start=false",
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
