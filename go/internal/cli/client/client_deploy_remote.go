package clientcmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

func ensureRemoteBinary(ctx context.Context, target sshTarget, localExe, remoteInstallDir string) error {
	remotePath := filepath.Join(remoteInstallDir, "xp2p.exe")

	if err := ensureRemoteDirectory(ctx, target, remoteInstallDir); err != nil {
		return err
	}

	exists, err := remotePathExists(ctx, target, remotePath)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	logging.Info("xp2p client deploy: uploading xp2p binary", "remote_path", remotePath, "version", version.Current())
	return scpCommandFunc(ctx, target, localExe, remotePath)
}

func prepareRemoteServer(ctx context.Context, target sshTarget, opts deployOptions) (string, error) {
	xp2pPath := filepath.Join(opts.remoteInstallDir, "xp2p.exe")
	present, err := remoteServerAssetsPresent(ctx, target, opts.remoteInstallDir, opts.remoteConfigDir)
	if err != nil {
		return "", err
	}
	if !present {
		args := []string{
			"server", "install",
			"--path", opts.remoteInstallDir,
			"--config-dir", opts.remoteConfigDir,
			"--port", opts.serverPort,
			"--host", opts.serverHost,
			"--force",
		}
		if _, err := remoteRunExecutable(ctx, target, xp2pPath, args); err != nil {
			return "", fmt.Errorf("server install: %w", err)
		}
	}

	if _, err := remoteRunExecutable(ctx, target, xp2pPath, []string{
		"server", "cert", "set",
		"--path", opts.remoteInstallDir,
		"--config-dir", opts.remoteConfigDir,
		"--host", opts.serverHost,
		"--force",
	}); err != nil {
		return "", fmt.Errorf("server cert set: %w", err)
	}

	rawOutput, err := remoteRunExecutable(ctx, target, xp2pPath, []string{
		"server", "user", "add",
		"--path", opts.remoteInstallDir,
		"--config-dir", opts.remoteConfigDir,
		"--id", opts.trojanUser,
		"--password", opts.trojanPassword,
		"--host", opts.serverHost,
	})
	if err != nil {
		return "", fmt.Errorf("server user add: %w", err)
	}

	link := extractTrojanLink(rawOutput)
	if link == "" {
		return "", fmt.Errorf("trojan link not found in server response")
	}

	return link, nil
}

func startRemoteServer(ctx context.Context, target sshTarget, opts deployOptions) error {
	xp2pPath := filepath.Join(opts.remoteInstallDir, "xp2p.exe")
	args := []string{
		"server", "run",
		"--path", opts.remoteInstallDir,
		"--config-dir", opts.remoteConfigDir,
		"--quiet",
		"--auto-install",
	}
	return remoteStartDetached(ctx, target, xp2pPath, args)
}

func ensureRemoteDirectory(ctx context.Context, target sshTarget, dir string) error {
	script := fmt.Sprintf("New-Item -Path %s -ItemType Directory -Force | Out-Null", psQuote(dir))
	_, err := sshInvokePowershell(ctx, target, script)
	return err
}

func remotePathExists(ctx context.Context, target sshTarget, path string) (bool, error) {
	script := fmt.Sprintf("if (Test-Path -LiteralPath %s) { Write-Output 'True' } else { Write-Output 'False' }", psQuote(path))
	out, err := sshInvokePowershell(ctx, target, script)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(out), "true"), nil
}

func remoteServerAssetsPresent(ctx context.Context, target sshTarget, installDir, configDir string) (bool, error) {
	binPath := filepath.Join(installDir, "bin", "xray.exe")
	script := fmt.Sprintf(`
$bin = %s
$cfg = %s
$required = @('inbounds.json', 'logs.json', 'outbounds.json', 'routing.json')
if (-not (Test-Path -LiteralPath $bin)) { Write-Output 'missing'; exit 0 }
if (-not (Test-Path -LiteralPath $cfg)) { Write-Output 'missing'; exit 0 }
foreach ($name in $required) {
  if (-not (Test-Path -LiteralPath (Join-Path $cfg $name))) { Write-Output 'missing'; exit 0 }
}
Write-Output 'present'`, psQuote(binPath), psQuote(resolveRemoteConfigPath(installDir, configDir)))

	out, err := sshInvokePowershell(ctx, target, script)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(out), "present"), nil
}

func resolveRemoteConfigPath(installDir, configDir string) string {
	cfg := strings.TrimSpace(configDir)
	if cfg == "" {
		cfg = server.DefaultServerConfigDir
	}
	if filepath.IsAbs(cfg) {
		return cfg
	}
	return filepath.Join(installDir, cfg)
}

func remoteRunExecutable(ctx context.Context, target sshTarget, exePath string, args []string) (string, error) {
	script := buildExecScript(exePath, args, true)
	out, err := sshInvokePowershell(ctx, target, script)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func remoteStartDetached(ctx context.Context, target sshTarget, exePath string, args []string) error {
	psArgs := make([]string, len(args))
	for i := range args {
		psArgs[i] = psArgQuote(args[i])
	}
	argList := strings.Join(psArgs, ", ")
	script := fmt.Sprintf("Start-Process -FilePath %s -ArgumentList %s -WindowStyle Hidden", psQuote(exePath), fmt.Sprintf("@(%s)", argList))
	_, err := sshInvokePowershell(ctx, target, script)
	return err
}

func extractTrojanLink(output string) string {
	for _, line := range strings.Split(output, "\n") {
		value := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(value), "trojan://") {
			return value
		}
	}
	return ""
}
