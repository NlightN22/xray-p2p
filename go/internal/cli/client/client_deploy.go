package clientcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func runClientDeploy(ctx context.Context, cfg config.Config, args []string) int {
	opts, err := parseDeployFlags(cfg, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		logging.Error("xp2p client deploy: argument parsing failed", "err", err)
		return 2
	}

	prereq, err := ensureSSHPrerequisitesFunc()
	if err != nil {
		logging.Error("xp2p client deploy: prerequisites failed", "err", err)
		return 1
	}
	opts.runtime.sshBinary = prereq.sshPath
	opts.runtime.scpBinary = prereq.scpPath

	packagePath, err := buildDeploymentPackageFunc(opts)
	if err != nil {
		logging.Error("xp2p client deploy: package preparation failed", "err", err)
		return 1
	}
	opts.packagePath = packagePath
	logging.Info("xp2p client deploy: package prepared", "path", packagePath)

	if opts.runtime.packageOnly {
		logging.Info("xp2p client deploy: package-only mode enabled, skipping remote deployment")
		return 0
	}

	if err := runRemoteDeploymentFunc(ctx, opts); err != nil {
		logging.Error("xp2p client deploy: remote deployment failed", "err", err)
		return 1
	}

	logging.Info("xp2p client deploy completed successfully")
	return 0
}

func parseDeployFlags(cfg config.Config, args []string) (deployOptions, error) {
	fs := flag.NewFlagSet("xp2p client deploy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	remoteHost := fs.String("remote-host", "", "SSH host name or address")
	sshUser := fs.String("ssh-user", "", "SSH user name")
	sshPort := fs.String("ssh-port", "22", "SSH port (default 22)")
	trojanUser := fs.String("user", "", "Trojan user identifier (email)")
	trojanPassword := fs.String("password", "", "Trojan user password (auto-generated when omitted)")
	trojanPort := fs.String("trojan-port", "", "Trojan service port")
	packageOnly := fs.Bool("package-only", false, "prepare deployment package only (skip remote operations)")

	if err := fs.Parse(args); err != nil {
		return deployOptions{}, err
	}
	if fs.NArg() > 0 {
		return deployOptions{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	host := strings.TrimSpace(*remoteHost)
	if host == "" || strings.HasPrefix(host, "-") {
		return deployOptions{}, fmt.Errorf("--remote-host is required")
	}
	if err := netutil.ValidateHost(host); err != nil {
		return deployOptions{}, fmt.Errorf("--remote-host: %v", err)
	}

	serverHostValue := firstNonEmpty(cfg.Server.Host, host)
	serverPortValue := normalizeServerPort(cfg, *trojanPort)

	packageOnlyValue := *packageOnly

	userValue := firstNonEmpty(*trojanUser, cfg.Client.User)
	if strings.TrimSpace(userValue) == "" {
		if packageOnlyValue {
			userValue = "client@example.invalid"
		} else {
			if promptStringFunc == nil {
				return deployOptions{}, fmt.Errorf("--user is required (set client.user in config to use default)")
			}
			value, err := promptStringFunc("Trojan user (email): ")
			if err != nil {
				return deployOptions{}, fmt.Errorf("prompt trojan user: %w", err)
			}
			userValue = strings.TrimSpace(value)
		}
	}
	if strings.TrimSpace(userValue) == "" {
		return deployOptions{}, fmt.Errorf("trojan user is required")
	}

	passwordValue := strings.TrimSpace(*trojanPassword)
	if passwordValue == "" {
		passwordValue = strings.TrimSpace(cfg.Client.Password)
	}
	if passwordValue == "" {
		if packageOnlyValue {
			passwordValue = "placeholder-secret"
		} else {
			gen, err := generateSecret(18)
			if err != nil {
				return deployOptions{}, fmt.Errorf("generate password: %w", err)
			}
			passwordValue = gen
		}
	}

	remoteInstallDir := firstNonEmpty(cfg.Server.InstallDir, defaultRemoteInstallDir)
	remoteConfigDir := firstNonEmpty(cfg.Server.ConfigDir, server.DefaultServerConfigDir)

	localInstallDir := firstNonEmpty(cfg.Client.InstallDir, defaultLocalInstallDir)
	localConfigDir := firstNonEmpty(cfg.Client.ConfigDir, client.DefaultClientConfigDir)

	return deployOptions{
		manifest: manifestOptions{
			remoteHost:     host,
			installDir:     strings.TrimSpace(remoteInstallDir),
			trojanPort:     serverPortValue,
			trojanUser:     strings.TrimSpace(userValue),
			trojanPassword: strings.TrimSpace(passwordValue),
		},
		runtime: runtimeOptions{
			remoteHost:      host,
			sshUser:         strings.TrimSpace(*sshUser),
			sshPort:         strings.TrimSpace(*sshPort),
			serverHost:      serverHostValue,
			remoteConfigDir: strings.TrimSpace(remoteConfigDir),
			localInstallDir: filepath.Clean(localInstallDir),
			localConfigDir:  strings.TrimSpace(localConfigDir),
			packageOnly:     packageOnlyValue,
		},
	}, nil
}
