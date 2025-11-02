//go:build windows

package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

// Run launches xray-core using the installed configuration directory and blocks until the process exits.
func Run(ctx context.Context, opts RunOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	configDir, err := resolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	xrayPath := filepath.Join(installDir, "bin", "xray.exe")
	if _, err := os.Stat(xrayPath); err != nil {
		return fmt.Errorf("xp2p: xray binary not found at %s: %w", xrayPath, err)
	}

	if stat, err := os.Stat(configDir); err != nil || !stat.IsDir() {
		if err != nil {
			return fmt.Errorf("xp2p: configuration directory not found at %s: %w", configDir, err)
		}
		return fmt.Errorf("xp2p: %s is not a directory", configDir)
	}

	args := []string{"-confdir", configDir}
	cmd := exec.CommandContext(ctx, xrayPath, args...)
	cmd.Dir = installDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
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

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamPipe(stdout, "stdout")
	}()
	go func() {
		defer wg.Done()
		streamPipe(stderr, "stderr")
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

func streamPipe(r io.Reader, stream string) {
	logger := logging.With("source", "xray_core", "stream", stream)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if stream == "stderr" {
			logger.Warn("xray_core output", "line", line)
		} else {
			logger.Info("xray_core output", "line", line)
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Warn("xray_core stream error", "err", err)
	}
}
