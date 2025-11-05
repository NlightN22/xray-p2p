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

	// Copy to the user's home directory without creating extra folders
	remoteParent := "."
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
		"$default = Join-Path -Path $package -ChildPath 'templates\\windows-amd64\\install.ps1'",
		"if (Test-Path -LiteralPath $default) { $scriptPath = $default } else {",
		"  $match = Get-ChildItem -LiteralPath $package -Recurse -Filter 'install.ps1' -ErrorAction SilentlyContinue | Select-Object -ExpandProperty FullName -First 1",
		"  if ($match) { $scriptPath = $match } else {",
		"    Write-Output '[INFO] xp2p: install.ps1 not found at default; listing top files:'",
		"    Get-ChildItem -LiteralPath $package -Recurse -File -ErrorAction SilentlyContinue | Select-Object -First 200 | ForEach-Object { Write-Output $_.FullName }",
		"    throw 'xp2p install script not found under ' + $package",
		"  }",
		"}",
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
		msg := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(stderr), strings.TrimSpace(stdout)}, "\n"))
		if msg != "" {
			return stdout, fmt.Errorf("%w: %s", err, msg)
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
