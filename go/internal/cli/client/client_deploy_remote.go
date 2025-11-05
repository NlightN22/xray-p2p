package clientcmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type remoteOS string

const (
	remoteOSWindows remoteOS = "windows"
	remoteOSLinux   remoteOS = "linux"
	remoteOSDarwin  remoteOS = "darwin"
	remoteOSUnknown remoteOS = "unknown"
)

type remoteWorkspace struct {
	baseDir       string
	packageParent string
	packageDir    string
}

func runRemoteDeployment(ctx context.Context, opts deployOptions) error {
	if strings.TrimSpace(opts.runtime.remoteHost) == "" {
		return errors.New("remote host is empty")
	}

	target := sshTarget{
		user: opts.runtime.sshUser,
		host: opts.runtime.remoteHost,
		port: opts.runtime.sshPort,
	}

	osID, err := detectRemoteOS(ctx, opts.runtime.sshBinary, target)
	if err != nil {
		return fmt.Errorf("detect remote operating system: %w", err)
	}

	switch osID {
	case remoteOSWindows:
		return runRemoteDeploymentWindows(ctx, opts, target)
	case remoteOSLinux, remoteOSDarwin:
		return fmt.Errorf("remote operating system %s is not supported yet", osID)
	default:
		return fmt.Errorf("remote operating system could not be determined")
	}
}

func detectRemoteOS(ctx context.Context, binary string, target sshTarget) (remoteOS, error) {
	if stdout, err := sshInvokePowershell(ctx, binary, target, "$PSVersionTable.PSEdition"); err == nil {
		if strings.TrimSpace(stdout) != "" {
			return remoteOSWindows, nil
		}
	}

	stdout, _, err := sshCommandFunc(ctx, binary, target, "uname -s")
	if err == nil {
		switch strings.ToLower(strings.TrimSpace(stdout)) {
		case "linux":
			return remoteOSLinux, nil
		case "darwin":
			return remoteOSDarwin, nil
		default:
			return remoteOSUnknown, nil
		}
	}

	return remoteOSUnknown, fmt.Errorf("unable to detect remote OS: %w", err)
}

func runRemoteDeploymentWindows(ctx context.Context, opts deployOptions, target sshTarget) error {
	workspace, err := prepareRemoteWorkspaceWindows(ctx, opts.runtime.sshBinary, target, opts.packagePath)
	if err != nil {
		return fmt.Errorf("prepare remote workspace: %w", err)
	}

	logging.Info("xp2p client deploy: uploading package",
		"local_path", opts.packagePath,
		"remote_base", workspace.baseDir,
	)

	if err := scpCopyFunc(ctx, opts.runtime.scpBinary, target, opts.packagePath, workspace.packageParent, true); err != nil {
		return fmt.Errorf("copy deployment package: %w", err)
	}

	logging.Info("xp2p client deploy: package uploaded",
		"remote_package_dir", workspace.packageDir,
	)

	output, err := runRemoteWindowsInstall(ctx, opts.runtime.sshBinary, target, workspace.packageDir)
	if err != nil {
		return fmt.Errorf("run remote install script: %w", err)
	}

	logging.Info("xp2p client deploy: windows install script completed",
		"remote_package_dir", workspace.packageDir,
		"output", strings.TrimSpace(output),
	)

	return nil
}

func prepareRemoteWorkspaceWindows(ctx context.Context, binary string, target sshTarget, packagePath string) (remoteWorkspace, error) {
	script := strings.Join([]string{
		"$base = Join-Path -Path ([IO.Path]::GetTempPath()) -ChildPath ('xp2p-client-' + [Guid]::NewGuid().ToString('N'))",
		"New-Item -ItemType Directory -Path $base -Force | Out-Null",
		"$packageParent = Join-Path -Path $base -ChildPath 'package'",
		"New-Item -ItemType Directory -Path $packageParent -Force | Out-Null",
		"$packageParent",
		"$base",
	}, "; ")

	output, err := sshInvokePowershell(ctx, binary, target, script)
	if err != nil {
		return remoteWorkspace{}, err
	}

	lines := splitNonEmptyLines(output)
	if len(lines) < 2 {
		return remoteWorkspace{}, fmt.Errorf("unexpected workspace response: %q", output)
	}

	baseName := filepath.Base(filepath.Clean(packagePath))
	if strings.TrimSpace(baseName) == "" || baseName == string(filepath.Separator) {
		return remoteWorkspace{}, fmt.Errorf("invalid package path %q", packagePath)
	}

	packageParent := lines[0]
	baseDir := lines[1]
	packageDir := joinWindowsPath(packageParent, baseName)

	return remoteWorkspace{
		baseDir:       baseDir,
		packageParent: packageParent,
		packageDir:    packageDir,
	}, nil
}

func runRemoteWindowsInstall(ctx context.Context, binary string, target sshTarget, packageDir string) (string, error) {
	packageLiteral := psQuote(packageDir)
	script := strings.Join([]string{
		fmt.Sprintf("$package = %s", packageLiteral),
		"$scriptPath = Join-Path -Path $package -ChildPath 'templates\\windows-amd64\\install.ps1'",
		"if (-not (Test-Path -LiteralPath $scriptPath)) { throw \"xp2p install script not found at $scriptPath\" }",
		"$scriptDir = Split-Path -Parent -LiteralPath $scriptPath",
		"Push-Location -LiteralPath $scriptDir",
		"try { & $scriptPath } finally { Pop-Location }",
	}, "; ")

	out, err := sshInvokePowershell(ctx, binary, target, script)
	if err != nil {
		return out, err
	}
	return out, nil
}

func splitNonEmptyLines(value string) []string {
	var lines []string
	replaced := strings.ReplaceAll(value, "\r\n", "\n")
	for _, line := range strings.Split(replaced, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func joinWindowsPath(first string, elements ...string) string {
	path := strings.TrimRight(strings.ReplaceAll(first, "/", `\`), `\`)
	for _, elem := range elements {
		trimmed := strings.Trim(strings.ReplaceAll(elem, "/", `\`), `\`)
		if trimmed == "" {
			continue
		}
		if path == "" {
			path = trimmed
		} else {
			path = path + `\` + trimmed
		}
	}
	return path
}
