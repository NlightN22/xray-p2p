//go:build windows

package server

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
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
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

	desired, err := loadServerDesiredConfig(installDir)
	if err != nil {
		return err
	}
	applied, err := loadServerAppliedState(filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName)))
	if err != nil {
		return err
	}
	if !applied.matches(desired.Reverse, desired.Redirects, desired.Forwards, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr) {
		if err := applyServerDesiredConfig(installDir, configDir, desired, applied.Reverse, ModeOptions{
			InstallDir: installDir,
			ConfigDir:  opts.ConfigDir,
			TunEnabled: opts.TunEnabled,
			TunName:    opts.TunName,
			TunMTU:     opts.TunMTU,
			TunAddr:    opts.TunAddr,
		}, true); err != nil {
			return err
		}
		if err := saveServerAppliedState(filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName)), desired.Reverse, desired.Redirects, desired.Forwards, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr); err != nil {
			return err
		}
	}

	if err := updateSendThroughOutbound(ctx, configDir, opts.TunEnabled); err != nil {
		return err
	}

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
			rel = "xp2p-server.log"
		}
		return filepath.Join(config.LogRoot(), rel), nil
	}

	configureCmd := func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		}
	}

	onStart := func() {
		if opts.TunEnabled {
			go winnet.DisableIPv6BindingWithRetry(ctx, opts.TunName)
			go func() {
				if err := applyRedirectRoutes(opts.TunName, opts.TunAddr, desired.Redirects); err != nil {
					logging.Warn("xp2p: redirect route setup failed", "err", err)
				}
			}()
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
