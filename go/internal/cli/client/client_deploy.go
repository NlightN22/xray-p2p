package clientcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

const deployStartupDelay = 2 * time.Second

func runClientDeploy(ctx context.Context, cfg config.Config, args []string) int {
	opts, err := parseDeployFlags(cfg, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		logging.Error("xp2p client deploy: argument parsing failed", "err", err)
		return 2
	}

	packagePath, err := buildDeploymentPackageFunc(opts)
	if err != nil {
		logging.Error("xp2p client deploy: package preparation failed", "err", err)
		return 1
	}
	opts.packagePath = packagePath
	logging.Info("xp2p client deploy: package prepared", "path", packagePath)

	if opts.packageOnly {
		logging.Info("xp2p client deploy: package-only mode enabled, skipping remote deployment")
		return 0
	}

	if err := ensureSSHPrerequisites(); err != nil {
		logging.Error("xp2p client deploy: prerequisites failed", "err", err)
		return 1
	}

	exePath, err := executablePathFunc()
	if err != nil {
		logging.Error("xp2p client deploy: resolve executable", "err", err)
		return 1
	}

	target := sshTarget{
		user: opts.sshUser,
		host: opts.remoteHost,
		port: opts.sshPort,
	}

	if err := ensureRemoteBinaryFunc(ctx, target, exePath, opts.remoteInstallDir); err != nil {
		logging.Error("xp2p client deploy: remote binary setup failed", "err", err)
		return 1
	}

	link, err := prepareRemoteServerFunc(ctx, target, opts)
	if err != nil {
		logging.Error("xp2p client deploy: remote provisioning failed", "err", err)
		return 1
	}

	logging.Info("xp2p client deploy: trojan link generated", "link", link)

	if err := installLocalClientFunc(ctx, opts, link); err != nil {
		logging.Error("xp2p client deploy: local installation failed", "err", err)
		return 1
	}

	var (
		startErr      error
		localProcess  *exec.Cmd
		remoteStarted bool
	)
	defer func() {
		if startErr != nil {
			if localProcess != nil {
				stopLocalProcessFunc(localProcess)
			}
			if remoteStarted {
				if err := stopRemoteFunc(ctx, target); err != nil {
					logging.Warn("xp2p client deploy: remote cleanup failed", "err", err)
				}
			}
		}
	}()

	if err := startRemoteServerFunc(ctx, target, opts); err != nil {
		startErr = err
		logging.Error("xp2p client deploy: unable to start remote server", "err", err)
		return 1
	}
	remoteStarted = true

	localCmd, err := startLocalClientFunc(opts)
	if err != nil {
		startErr = err
		logging.Error("xp2p client deploy: unable to start local client", "err", err)
		return 1
	}
	localProcess = localCmd

	waitForTunnelStartup()

	if err := runPingCheckFunc(ctx, opts); err != nil {
		startErr = err
		logging.Error("xp2p client deploy: connectivity check failed", "err", err)
		return 1
	}

	releaseProcessHandleFunc(localProcess)
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
	serverPortValue := normalizeServerPort(cfg, "")

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
		remoteHost:       host,
		sshUser:          strings.TrimSpace(*sshUser),
		sshPort:          strings.TrimSpace(*sshPort),
		serverHost:       serverHostValue,
		serverPort:       serverPortValue,
		trojanUser:       strings.TrimSpace(userValue),
		trojanPassword:   strings.TrimSpace(passwordValue),
		remoteInstallDir: strings.TrimSpace(remoteInstallDir),
		remoteConfigDir:  strings.TrimSpace(remoteConfigDir),
		localInstallDir:  filepath.Clean(localInstallDir),
		localConfigDir:   strings.TrimSpace(localConfigDir),
		packageOnly:      packageOnlyValue,
	}, nil
}
