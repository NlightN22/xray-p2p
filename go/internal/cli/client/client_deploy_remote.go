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

	remoteParent := "~"
	remotePackageDir := remoteParent + "/" + baseName

	logging.Info("xp2p client deploy: uploading package",
		"local_path", opts.packagePath,
		"remote_parent", remoteParent,
	)

	if err := scpCopyFunc(ctx, opts.runtime.scpBinary, target, opts.packagePath, remoteParent, true); err != nil {
		return fmt.Errorf("copy deployment package: %w", err)
	}

	logging.Info("xp2p client deploy: package uploaded", "remote_package_dir", remotePackageDir)

	// Try PowerShell first; if unavailable, fallback to POSIX sh in a single SSH command.
	output, err := runRemoteInstallCombined(ctx, opts.runtime.sshBinary, target, baseName)
	if err != nil {
		return fmt.Errorf("run remote install script: %w", err)
	}
	logging.Info("xp2p client deploy: install script completed", "output", strings.TrimSpace(output))
	return nil
}

// runRemoteInstallCombined builds one command that tries PowerShell first, then falls back to POSIX sh.
func runRemoteInstallCombined(ctx context.Context, binary string, target sshTarget, packageBaseName string) (string, error) {
	psScript := strings.Join([]string{
		fmt.Sprintf("$package = Join-Path -Path $HOME -ChildPath %s", psQuote(packageBaseName)),
		"$scriptPath = Join-Path -Path $package -ChildPath 'templates\\windows-amd64\\install.ps1'",
		"if (-not (Test-Path -LiteralPath $scriptPath)) { throw \"xp2p install script not found at $scriptPath\" }",
		"$scriptDir = Split-Path -Parent -LiteralPath $scriptPath",
		"Push-Location -LiteralPath $scriptDir",
		"try { & $scriptPath } finally { Pop-Location }",
	}, "; ")
	encoded := encodePowershellCommand(fmt.Sprintf("& { %s }", psScript))

	shLines := []string{
		fmt.Sprintf("PACKAGE=\"$HOME/%s\"", escapeForSh(packageBaseName)),
		"if [ -f \"$PACKAGE/templates/linux-amd64/install.sh\" ]; then sh \"$PACKAGE/templates/linux-amd64/install.sh\"; exit $?; fi",
		"if [ -f \"$PACKAGE/templates/darwin-amd64/install.sh\" ]; then sh \"$PACKAGE/templates/darwin-amd64/install.sh\"; exit $?; fi",
		"uname_s=\"$(uname -s 2>/dev/null || echo unknown)\"",
		"case \"$uname_s\" in",
		"  Linux)  sh \"$PACKAGE/templates/linux-amd64/install.sh\" ;;",
		"  Darwin) sh \"$PACKAGE/templates/darwin-amd64/install.sh\" ;;",
		"  *) echo \"xp2p: no suitable installer for $uname_s\" 1>&2; exit 1 ;;",
		"esac",
	}
	shScript := strings.Join(shLines, "; ")
	combined := fmt.Sprintf("powershell -NoLogo -NoProfile -NonInteractive -EncodedCommand %s || sh -c %s", encoded, shQuote(shScript))

	stdout, stderr, err := sshCommandFunc(ctx, binary, target, combined)
	if err != nil {
		if strings.TrimSpace(stderr) != "" {
			return stdout, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
		}
		return stdout, err
	}
	return strings.TrimSpace(stdout), nil
}

func shQuote(s string) string {
	if s == "" {
		return "''"
	}
	// Replace ' with '\'' and wrap result in single quotes
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func escapeForSh(s string) string {
	// Minimal escaping for inclusion inside double quotes
	return strings.ReplaceAll(s, "\"", "\\\"")
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
