//go:build linux

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/linuxnet"
	"github.com/NlightN22/xray-p2p/go/internal/openwrt"
	"github.com/NlightN22/xray-p2p/go/internal/xray"
)

// Run launches xray-core using the installed server configuration directory and blocks until completion.
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

	if stat, err := os.Stat(configDir); err != nil || !stat.IsDir() {
		if err != nil {
			return fmt.Errorf("xp2p: configuration directory not found at %s: %w", configDir, err)
		}
		return fmt.Errorf("xp2p: %s is not a directory", configDir)
	}

	if opts.TunEnabled {
		if err := openwrt.EnsureTunInterface(opts.TunName, opts.TunAddr); err != nil {
			return err
		}
		if err := linuxnet.EnsureTunInterface(opts.TunName, opts.TunAddr, opts.TunMTU); err != nil {
			return err
		}
	}

	xrayPath, err := xray.ResolveBinaryPath()
	if err != nil {
		return err
	}
	return runXrayWithConfig(
		ctx,
		xrayPath,
		configDir,
		configDir,
		opts.ErrorLogPath,
		resolveServerLogPath,
		nil,
	)
}

func resolveServerLogPath(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("xp2p: log path is empty")
	}
	trimmed := strings.TrimSpace(filepath.Clean(raw))
	if trimmed == "" || trimmed == "." {
		return "", errors.New("xp2p: log path is empty")
	}
	if filepath.IsAbs(trimmed) {
		return trimmed, nil
	}
	rel := filepath.ToSlash(trimmed)
	rel = strings.TrimPrefix(rel, "logs/")
	if rel == "" || rel == "." {
		rel = "xp2p-server.log"
	}
	return filepath.Join(layout.UnixLogRoot, rel), nil
}
