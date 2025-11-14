package clientcmd

import (
	"context"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestRunClientRemoveAll(t *testing.T) {
	cfg := clientCfg(`C:\xp2p-client`, client.DefaultClientConfigDir)
	args := []string{"--all", "--keep-files", "--ignore-missing"}

	code, capture := execClientRemove(cfg, args)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if !capture.removeCalled {
		t.Fatalf("expected remove all to be called")
	}
	want := client.RemoveOptions{
		InstallDir:    cfg.Client.InstallDir,
		ConfigDir:     cfg.Client.ConfigDir,
		KeepFiles:     true,
		IgnoreMissing: true,
	}
	requireEqual(t, capture.removeOpts, want, "remove options")
	if capture.endpointCalled {
		t.Fatalf("endpoint removal should not be called")
	}
}

func TestRunClientRemoveEndpoint(t *testing.T) {
	cfg := clientCfg(`C:\xp2p-client`, client.DefaultClientConfigDir)
	args := []string{"example.com"}

	code, capture := execClientRemove(cfg, args)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if !capture.endpointCalled {
		t.Fatalf("expected endpoint removal to be called")
	}
	want := client.RemoveEndpointOptions{
		InstallDir: cfg.Client.InstallDir,
		ConfigDir:  cfg.Client.ConfigDir,
		Target:     "example.com",
	}
	requireEqual(t, capture.endpointOpts, want, "endpoint remove options")
	if capture.removeCalled {
		t.Fatalf("remove all should not be called")
	}
}

func TestRunClientRemoveRequiresArgument(t *testing.T) {
	cfg := clientCfg(`C:\xp2p-client`, client.DefaultClientConfigDir)

	code, capture := execClientRemove(cfg, []string{})
	if code != 2 {
		t.Fatalf("exit code: got %d want 2", code)
	}
	if capture.removeCalled || capture.endpointCalled {
		t.Fatalf("no removal should be invoked when arguments are invalid")
	}
}

type removeCapture struct {
	removeCalled   bool
	endpointCalled bool
	removeOpts     client.RemoveOptions
	endpointOpts   client.RemoveEndpointOptions
}

func execClientRemove(cfg config.Config, args []string) (int, removeCapture) {
	var capture removeCapture

	restoreInstall := stubClientInstall(nil)
	defer restoreInstall()

	restoreRemove := stubClientRemove(func(ctx context.Context, opts client.RemoveOptions) error {
		capture.removeCalled = true
		capture.removeOpts = opts
		return nil
	})
	defer restoreRemove()

	restoreEndpoint := stubClientRemoveEndpoint(func(ctx context.Context, opts client.RemoveEndpointOptions) error {
		capture.endpointCalled = true
		capture.endpointOpts = opts
		return nil
	})
	defer restoreEndpoint()

	code := runClientRemove(context.Background(), cfg, args)
	return code, capture
}
