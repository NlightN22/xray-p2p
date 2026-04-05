package client

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/xray"
)

type cmdConfigurator func(*exec.Cmd)
type startHook func() error
type readyCheck func(context.Context) error

func runXrayWithConfig(
	ctx context.Context,
	xrayPath string,
	configDir string,
	cmdDir string,
	configureCmd cmdConfigurator,
	onStart startHook,
	onReady readyCheck,
) error {
	var errorWriter io.Writer

	if err := xray.VerifyPinnedVersion(ctx, xrayPath); err != nil {
		return err
	}

	args := []string{"-confdir", configDir}
	cmd := exec.CommandContext(ctx, xrayPath, args...)
	cmd.Dir = cmdDir
	if configureCmd != nil {
		configureCmd(cmd)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("xp2p: capture stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("xp2p: capture stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("xp2p: start xray-core: %w", err)
	}

	logging.Info("xray-core process started", "path", xrayPath)
	if onReady != nil {
		if err := onReady(ctx); err != nil {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
			return fmt.Errorf("xp2p: xray-core health check failed: %w", err)
		}
	}
	if onStart != nil {
		if err := onStart(); err != nil {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
			return err
		}
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

	waitErr := cmd.Wait()
	wg.Wait()

	if ctx.Err() != nil {
		logging.Info("xray-core process terminated due to context cancel")
		return nil
	}
	if waitErr != nil {
		return fmt.Errorf("xp2p: xray-core exited: %w", waitErr)
	}
	return nil
}
