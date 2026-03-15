package servercmd

import (
	"context"
	"fmt"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

// InstallOptions describes xp2p server install parameters for UI integrations.
type InstallOptions struct {
	Path      string
	ConfigDir string
	Port      string
	CertStore string
	CertFile  string
	KeyFile   string
	Host      string
}

// DeployOptions describes xp2p server deploy parameters for UI integrations.
type DeployOptions struct {
	Listen   string
	Link     string
	DiagPort string
	Timeout  time.Duration
}

// Install runs the server install workflow without invoking the CLI.
func Install(ctx context.Context, cfg config.Config, opts InstallOptions) error {
	commandOpts := serverInstallCommandOptions{
		Path:      opts.Path,
		ConfigDir: opts.ConfigDir,
		Port:      opts.Port,
		CertStore: opts.CertStore,
		Cert:      opts.CertFile,
		Key:       opts.KeyFile,
		Host:      opts.Host,
		Force:     true,
	}
	if code := runServerInstall(ctx, cfg, commandOpts); code != 0 {
		return fmt.Errorf("xp2p server install failed")
	}
	return nil
}

// Deploy runs the server deploy workflow without invoking the CLI.
func Deploy(ctx context.Context, cfg config.Config, opts DeployOptions) error {
	commandOpts := serverDeployOptions{
		Listen:   opts.Listen,
		Link:     opts.Link,
		DiagPort: opts.DiagPort,
		Once:     true,
		Timeout:  opts.Timeout,
	}
	if code := runServerDeploy(ctx, cfg, commandOpts); code != 0 {
		return fmt.Errorf("xp2p server deploy failed")
	}
	return nil
}
