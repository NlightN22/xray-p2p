package clientcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

// DeployOptions describes xp2p client deploy parameters for UI integrations.
type DeployOptions struct {
	Host       string
	DeployPort string
	InstallDir string
	User       string
	Password   string
	TrojanPort string
}

// Deploy runs the client deploy workflow without invoking the CLI.
func Deploy(ctx context.Context, cfg config.Config, opts DeployOptions) error {
	host := strings.TrimSpace(opts.Host)
	if host == "" {
		return fmt.Errorf("deploy host is required")
	}

	args := buildDeployArgs(opts)
	code := runClientDeploy(ctx, cfg, args)
	if code != 0 {
		return fmt.Errorf("xp2p client deploy failed")
	}
	return nil
}

func buildDeployArgs(opts DeployOptions) []string {
	args := make([]string, 0, 10)
	if value := strings.TrimSpace(opts.Host); value != "" {
		args = append(args, "--host="+value)
	}
	if value := strings.TrimSpace(opts.DeployPort); value != "" {
		args = append(args, "--port="+value)
	}
	if value := strings.TrimSpace(opts.InstallDir); value != "" {
		args = append(args, "--install-dir="+value)
	}
	if value := strings.TrimSpace(opts.User); value != "" {
		args = append(args, "--user="+value)
	}
	if value := strings.TrimSpace(opts.Password); value != "" {
		args = append(args, "--password="+value)
	}
	if value := strings.TrimSpace(opts.TrojanPort); value != "" {
		args = append(args, "--trojan-port="+value)
	}
	return args
}
