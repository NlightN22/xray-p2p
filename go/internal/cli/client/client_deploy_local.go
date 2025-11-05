package clientcmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const localTunnelStartupDelay = 2 * time.Second

func installLocalClient(ctx context.Context, opts deployOptions, link string) error {
	linkData, err := parseTrojanLink(link)
	if err != nil {
		return fmt.Errorf("parse trojan link: %w", err)
	}

	installOpts := client.InstallOptions{
		InstallDir:    opts.runtime.localInstallDir,
		ConfigDir:     opts.runtime.localConfigDir,
		ServerAddress: linkData.ServerAddress,
		ServerPort:    linkData.ServerPort,
		User:          linkData.User,
		Password:      linkData.Password,
		ServerName:    linkData.ServerName,
		AllowInsecure: linkData.AllowInsecure,
		Force:         true,
	}

	if err := clientInstallFunc(ctx, installOpts); err != nil {
		return err
	}

	logging.Info("xp2p client deploy: local client installed",
		"install_dir", opts.runtime.localInstallDir,
		"config_dir", opts.runtime.localConfigDir,
	)
	return nil
}

func startLocalClient(opts deployOptions) (*exec.Cmd, error) {
	exe, err := executablePathFunc()
	if err != nil {
		return nil, err
	}

	args := []string{
		"client", "run",
		"--path", opts.runtime.localInstallDir,
		"--config-dir", opts.runtime.localConfigDir,
		"--quiet",
		"--auto-install",
	}

	return startProcessFunc(exe, args)
}

func runPingCheck(ctx context.Context, opts deployOptions) error {
	exe, err := executablePathFunc()
	if err != nil {
		return err
	}

	args := []string{
		"ping", defaultPingTarget,
		"--socks",
	}

	output, err := runPingCommandFunc(ctx, exe, args)
	if err != nil {
		return fmt.Errorf("xp2p ping failed: %w; output: %s", err, strings.TrimSpace(string(output)))
	}

	logging.Info("xp2p client deploy: ping succeeded", "output", strings.TrimSpace(string(output)))
	return nil
}

func runPingCommand(ctx context.Context, exe string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	return cmd.CombinedOutput()
}

func waitForTunnelStartup() {
	sleepFunc(localTunnelStartupDelay)
}

func releaseDetachedProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Release()
}
