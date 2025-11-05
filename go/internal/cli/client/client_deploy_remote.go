package clientcmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func runRemoteDeployment(ctx context.Context, opts deployOptions) error {
	if strings.TrimSpace(opts.runtime.remoteHost) == "" {
		return errors.New("remote host is empty")
	}

	target := sshTarget{
		user: opts.runtime.sshUser,
		host: opts.runtime.remoteHost,
		port: opts.runtime.sshPort,
	}

	baseName := filepath.Base(filepath.Clean(opts.packagePath))
	if strings.TrimSpace(baseName) == "" {
		return fmt.Errorf("invalid package path %q", opts.packagePath)
	}

	remoteParent := "~/.xp2p-deploy"
	remotePackageDir := remoteParent + "/" + baseName

	logging.Info("xp2p client deploy: uploading package",
		"local_path", opts.packagePath,
		"remote_parent", remoteParent,
	)

	if err := scpCopyFunc(ctx, opts.runtime.scpBinary, target, opts.packagePath, remoteParent, true); err != nil {
		return fmt.Errorf("copy deployment package: %w", err)
	}

	logging.Info("xp2p client deploy: package uploaded", "remote_package_dir", remotePackageDir)

	// Windows-only execution for now: run PowerShell installer under user's HOME.
	pathExpr := windowsHomeJoin(".xp2p-deploy", baseName)
	output, err := runRemoteWindowsInstall(ctx, opts.runtime.sshBinary, target, pathExpr)
	if err != nil {
		return fmt.Errorf("run remote install script: %w", err)
	}
	logging.Info("xp2p client deploy: install script completed", "output", strings.TrimSpace(output))
	return nil
}

// windowsHomeJoin builds a PowerShell Join-Path expression that joins $HOME with provided parts.
func windowsHomeJoin(parts ...string) string {
	var quoted []string
	for _, p := range parts {
		quoted = append(quoted, psQuote(strings.ReplaceAll(p, "/", `\`)))
	}
	// $HOME, 'part1', 'part2'
	return "$HOME, " + strings.Join(quoted, ", ")
}

func runRemoteWindowsInstall(ctx context.Context, binary string, target sshTarget, packagePathExpr string) (string, error) {
	// packagePathExpr is a PS expression, not a quoted string.
	script := strings.Join([]string{
		fmt.Sprintf("$package = Join-Path -Path %s", packagePathExpr),
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
