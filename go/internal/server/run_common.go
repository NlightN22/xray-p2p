package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/xray"
)

type logPathResolver func(string) (string, error)
type cmdConfigurator func(*exec.Cmd)
type startHook func()

func runXrayWithConfig(
	ctx context.Context,
	xrayPath string,
	configDir string,
	cmdDir string,
	errorLogPath string,
	resolveLogPath logPathResolver,
	configureCmd cmdConfigurator,
	onStart startHook,
) error {
	if err := xray.VerifyPinnedVersion(ctx, xrayPath); err != nil {
		return err
	}

	var errorWriter io.Writer
	var errorFile *os.File
	if raw := strings.TrimSpace(errorLogPath); raw != "" {
		logPath, err := resolveLogPath(raw)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			return fmt.Errorf("xp2p: create log directory %s: %w", filepath.Dir(logPath), err)
		}
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("xp2p: open xray log file %s: %w", logPath, err)
		}
		errorFile = file
		errorWriter = file
		defer func() { _ = errorFile.Close() }()
		logging.Info("xray-core stderr redirected to file", "path", logPath)
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
	if onStart != nil {
		onStart()
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
