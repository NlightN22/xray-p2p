package server

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/xray"
	"github.com/NlightN22/xray-p2p/go/internal/xrayguard"
)

type cmdConfigurator func(*exec.Cmd)
type startHook func() error
type readyCheck func(context.Context) error
type guardHook func(xrayguard.Event)

var monitorXrayProcess = xrayguard.Monitor

type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func newTailBuffer(max int) *tailBuffer {
	return &tailBuffer{max: max}
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.max <= 0 {
		return len(p), nil
	}
	if len(p) >= t.max {
		t.buf = append([]byte(nil), p[len(p)-t.max:]...)
		return len(p), nil
	}
	if len(t.buf)+len(p) > t.max {
		drop := len(t.buf) + len(p) - t.max
		t.buf = append(t.buf[drop:], p...)
		return len(p), nil
	}
	t.buf = append(t.buf, p...)
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func runXrayWithConfig(
	ctx context.Context,
	xrayPath string,
	configPath string,
	cmdDir string,
	configureCmd cmdConfigurator,
	onStart startHook,
	onReady readyCheck,
	guardHooks ...guardHook,
) error {
	var errorWriter io.Writer

	if err := xray.VerifyPinnedVersion(ctx, xrayPath); err != nil {
		return err
	}

	args := []string{"-config", configPath}
	cmd := exec.CommandContext(ctx, xrayPath, args...)
	cmd.Dir = cmdDir
	if configureCmd != nil {
		configureCmd(cmd)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("capture stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("capture stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start xray-core: %w", err)
	}
	pid := cmd.Process.Pid

	stderrTail := newTailBuffer(16 * 1024)
	if errorWriter != nil {
		errorWriter = io.MultiWriter(errorWriter, stderrTail)
	} else {
		errorWriter = stderrTail
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamPipe(stdout, "stdout", nil)
	}()
	go func() {
		defer wg.Done()
		streamPipe(stderr, "stderr", errorWriter)
	}()

	procExit := make(chan struct{})
	var waitErr error
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		waitErr = cmd.Wait()
		close(procExit)
	}()

	guardCtx, cancelGuard := context.WithCancel(ctx)
	workers.Add(1)
	var guardMu sync.Mutex
	var guardEvent *xrayguard.Event
	go func() {
		defer workers.Done()
		eventCh := monitorXrayProcess(guardCtx, pid, xrayguard.DefaultOptions())
		for event := range eventCh {
			guardMu.Lock()
			copied := event
			guardEvent = &copied
			guardMu.Unlock()

			logging.Error("xray-core loop protection triggered",
				"reason", event.Reason,
				"pid", event.PID,
				"fd_before", event.Before.FDCount,
				"fd_after", event.After.FDCount,
				"fd_delta", event.FDDelta,
				"window", event.Window.String(),
				"socket_ratio_percent", event.SocketRatioPercent,
				"established_tcp", event.EstablishedTCPCount,
				"action", event.Action,
			)
			for _, hook := range guardHooks {
				if hook != nil {
					hook(event)
				}
			}
			if cmd.Process != nil && !isClosed(procExit) {
				_ = cmd.Process.Kill()
			}
			return
		}
	}()

	readyCtx, cancelReady := context.WithCancel(ctx)
	workers.Add(1)
	go func() {
		defer workers.Done()
		<-procExit
		cancelReady()
	}()
	defer func() {
		cancelReady()
		cancelGuard()
		if !isClosed(procExit) && cmd.Process != nil {
			_ = cmd.Process.Kill()
			<-procExit
		}
		wg.Wait()
		workers.Wait()
	}()

	logging.Info("xray-core process started", "path", xrayPath)
	if onReady != nil {
		if err := onReady(readyCtx); err != nil {
			if ctx.Err() != nil {
				if cmd.Process != nil && !isClosed(procExit) {
					_ = cmd.Process.Kill()
				}
				<-procExit
				wg.Wait()
				logging.Info("xray-core process terminated due to context cancel")
				return nil
			}
			if isClosed(procExit) {
				wg.Wait()
				guardMu.Lock()
				event := guardEvent
				guardMu.Unlock()
				if event != nil {
					return *event
				}
				tail := strings.TrimSpace(stderrTail.String())
				if tail != "" {
					return fmt.Errorf("xray-core exited before ready: %w (stderr tail: %s)", waitErr, tail)
				}
				return fmt.Errorf("xray-core exited before ready: %w", waitErr)
			}

			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-procExit
			wg.Wait()
			return fmt.Errorf("xray-core health check failed: %w", err)
		}
	}
	if onStart != nil {
		if err := onStart(); err != nil {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-procExit
			wg.Wait()
			return err
		}
	}

	<-procExit
	cancelGuard()
	wg.Wait()

	if ctx.Err() != nil {
		logging.Info("xray-core process terminated due to context cancel")
		return nil
	}
	if waitErr != nil {
		guardMu.Lock()
		event := guardEvent
		guardMu.Unlock()
		if event != nil {
			return *event
		}
		tail := strings.TrimSpace(stderrTail.String())
		if tail != "" {
			return fmt.Errorf("xray-core exited: %w (stderr tail: %s)", waitErr, tail)
		}
		return fmt.Errorf("xray-core exited: %w", waitErr)
	}
	return nil
}
