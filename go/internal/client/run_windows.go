//go:build windows

package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

// Run launches xray-core using the installed client configuration directory and blocks until the process exits.
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

	xrayPath := filepath.Join(installDir, layout.BinDirName, "xray.exe")
	if _, err := os.Stat(xrayPath); err != nil {
		return fmt.Errorf("xp2p: xray binary not found at %s: %w", xrayPath, err)
	}

	if stat, err := os.Stat(configDir); err != nil || !stat.IsDir() {
		if err != nil {
			return fmt.Errorf("xp2p: configuration directory not found at %s: %w", configDir, err)
		}
		return fmt.Errorf("xp2p: %s is not a directory", configDir)
	}

	stopHeartbeat := startHeartbeatLoop(ctx, installDir, opts.Heartbeat)
	defer stopHeartbeat()

	var errorWriter io.Writer
	var errorFile *os.File
	if path := strings.TrimSpace(opts.ErrorLogPath); path != "" {
		logPath := path
		if !filepath.IsAbs(logPath) {
			logPath = filepath.Join(installDir, logPath)
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
	cmd.Dir = installDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
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
