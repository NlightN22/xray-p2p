package client

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/xrayguard"
)

func TestClientRunXrayJoinsWorkersOnCancellation(t *testing.T) {
	t.Setenv("XP2P_XRAY_SKIP_VERSION_CHECK", "1")
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	go func() {
		<-ready
		cancel()
	}()
	err := runXrayWithConfig(ctx, os.Args[0], "", "", clientHelperCommand("wait"), nil, func(context.Context) error {
		close(ready)
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("runXrayWithConfig cancellation error: %v", err)
	}
}

func TestClientRunXrayJoinsWorkersOnReadinessFailure(t *testing.T) {
	t.Setenv("XP2P_XRAY_SKIP_VERSION_CHECK", "1")
	want := errors.New("not ready")
	err := runXrayWithConfig(context.Background(), os.Args[0], "", "", clientHelperCommand("wait"), nil, func(context.Context) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("runXrayWithConfig error = %v, want %v", err, want)
	}
}

func TestClientRunXrayJoinsWorkersOnGuardTrigger(t *testing.T) {
	t.Setenv("XP2P_XRAY_SKIP_VERSION_CHECK", "1")
	previous := monitorXrayProcess
	t.Cleanup(func() { monitorXrayProcess = previous })
	monitorXrayProcess = func(context.Context, int, xrayguard.Options) <-chan xrayguard.Event {
		events := make(chan xrayguard.Event, 1)
		events <- xrayguard.Event{Reason: xrayguard.ReasonFDSpike}
		close(events)
		return events
	}
	err := runXrayWithConfig(context.Background(), os.Args[0], "", "", clientHelperCommand("wait"), nil, nil)
	var event xrayguard.Event
	if !errors.As(err, &event) {
		t.Fatalf("runXrayWithConfig error = %v, want guard event", err)
	}
}

func clientHelperCommand(mode string) cmdConfigurator {
	return func(cmd *exec.Cmd) {
		if runtime.GOOS == "windows" {
			path, _ := exec.LookPath("powershell.exe")
			cmd.Path = path
			script := "exit 0"
			if mode == "wait" {
				script = "Start-Sleep -Seconds 30"
			}
			cmd.Args = []string{path, "-NoProfile", "-NonInteractive", "-Command", script}
			return
		}
		path, _ := exec.LookPath("sh")
		cmd.Path = path
		script := "exit 0"
		if mode == "wait" {
			script = "sleep 30"
		}
		cmd.Args = []string{path, "-c", script}
	}
}
