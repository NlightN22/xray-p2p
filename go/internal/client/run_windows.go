//go:build windows

package client

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
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

	paths, err := resolveClientPaths(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	desired, err := loadClientInstallState(paths.configFile)
	if err != nil {
		return err
	}
	applied, err := loadClientAppliedState(paths.stateFile)
	if err != nil {
		return err
	}
	if !applied.matches(desired, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr) {
		if err := applyClientDesiredConfig(paths, desired, ModeOptions{
			InstallDir: installDir,
			ConfigDir:  opts.ConfigDir,
			TunEnabled: opts.TunEnabled,
			TunName:    opts.TunName,
			TunMTU:     opts.TunMTU,
			TunAddr:    opts.TunAddr,
		}, true); err != nil {
			return err
		}
		if err := saveClientAppliedState(paths.stateFile, desired, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr); err != nil {
			return err
		}
	}

	if err := updateSendThroughOutbound(ctx, paths, opts.TunEnabled); err != nil {
		return err
	}

	stopHeartbeat := startHeartbeatLoop(ctx, installDir, configDir, opts.Heartbeat)
	defer stopHeartbeat()

	resolveLogPath := func(raw string) (string, error) {
		if strings.TrimSpace(raw) == "" {
			return "", fmt.Errorf("xp2p: log path is empty")
		}
		trimmed := strings.TrimSpace(filepath.Clean(raw))
		if trimmed == "" || trimmed == "." {
			return "", fmt.Errorf("xp2p: log path is empty")
		}
		if filepath.IsAbs(trimmed) {
			return trimmed, nil
		}
		rel := filepath.ToSlash(trimmed)
		rel = strings.TrimPrefix(rel, "logs/")
		if rel == "" || rel == "." {
			rel = "xp2p-client.log"
		}
		return filepath.Join(config.LogRoot(), rel), nil
	}

	configureCmd := func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
		}
	}

	onStart := func() {
		if opts.TunEnabled {
			go winnet.DisableIPv6BindingWithRetry(ctx, opts.TunName)
		}
	}

	return runXrayWithConfig(
		ctx,
		xrayPath,
		configDir,
		installDir,
		opts.ErrorLogPath,
		resolveLogPath,
		configureCmd,
		onStart,
	)
}
