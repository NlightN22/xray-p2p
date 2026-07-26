package server

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/xrayguard"
)

func TestServerRunXrayJoinsWorkersOnProcessExit(t *testing.T) {
	t.Setenv("XP2P_XRAY_SKIP_VERSION_CHECK", "1")
	err := runXrayWithConfig(context.Background(), os.Args[0], "", "", serverHelperCommand("exit"), nil, nil)
	if err != nil {
		t.Fatalf("runXrayWithConfig process exit error: %v", err)
	}
}

func TestServerRunXrayJoinsWorkersOnStartHookFailure(t *testing.T) {
	t.Setenv("XP2P_XRAY_SKIP_VERSION_CHECK", "1")
	want := errors.New("start hook failed")
	err := runXrayWithConfig(context.Background(), os.Args[0], "", "", serverHelperCommand("wait"), func() error {
		return want
	}, nil)
	if !errors.Is(err, want) {
		t.Fatalf("runXrayWithConfig error = %v, want %v", err, want)
	}
}

func TestServerRunXrayJoinsWorkersOnGuardTrigger(t *testing.T) {
	t.Setenv("XP2P_XRAY_SKIP_VERSION_CHECK", "1")
	previous := monitorXrayProcess
	t.Cleanup(func() { monitorXrayProcess = previous })
	monitorXrayProcess = func(context.Context, int, xrayguard.Options) <-chan xrayguard.Event {
		events := make(chan xrayguard.Event, 1)
		events <- xrayguard.Event{Reason: xrayguard.ReasonFDSpike}
		close(events)
		return events
	}
	err := runXrayWithConfig(context.Background(), os.Args[0], "", "", serverHelperCommand("wait"), nil, nil)
	var event xrayguard.Event
	if !errors.As(err, &event) {
		t.Fatalf("runXrayWithConfig error = %v, want guard event", err)
	}
}

func serverHelperCommand(mode string) cmdConfigurator {
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
