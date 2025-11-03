package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf16"
)

func buildExecScript(exePath string, args []string, enforceExit bool) string {
	var parts []string
	parts = append(parts, "&", psQuote(exePath))
	for _, arg := range args {
		parts = append(parts, psArgQuote(arg))
	}
	command := strings.Join(parts, " ")
	if enforceExit {
		return fmt.Sprintf("%s; exit $LASTEXITCODE", command)
	}
	return command
}

func psArgQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.ContainsAny(value, " `'\"") {
		return psQuote(value)
	}
	return value
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func sshInvokePowershell(ctx context.Context, target sshTarget, script string) (string, error) {
	encoded := encodePowershellCommand(fmt.Sprintf("& { %s }", script))
	command := fmt.Sprintf("powershell -NoLogo -NoProfile -NonInteractive -EncodedCommand %s", encoded)
	stdout, stderr, err := sshCommandFunc(ctx, target, command)
	if err != nil {
		if strings.TrimSpace(stderr) != "" {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
		}
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func runSSHCommand(ctx context.Context, target sshTarget, command string) (string, string, error) {
	args := buildSSHArgs(target, command)
	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func runSCPCommand(ctx context.Context, target sshTarget, localPath, remotePath string) error {
	args := buildSCPArgs(target, localPath, remotePath)
	cmd := exec.CommandContext(ctx, "scp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.TrimSpace(stderr.String()) != "" {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return err
	}
	return nil
}

func buildSSHArgs(target sshTarget, command string) []string {
	var args []string
	if target.port != "" {
		args = append(args, "-p", target.port)
	}
	args = append(args, targetAddress(target), command)
	return args
}

func buildSCPArgs(target sshTarget, localPath, remotePath string) []string {
	var args []string
	if target.port != "" {
		args = append(args, "-P", target.port)
	}
	args = append(args, localPath, fmt.Sprintf("%s:%s", targetAddress(target), scpPath(remotePath)))
	return args
}

func scpPath(path string) string {
	return strings.ReplaceAll(path, `\`, `/`)
}

func targetAddress(target sshTarget) string {
	if strings.TrimSpace(target.user) == "" {
		return target.host
	}
	return fmt.Sprintf("%s@%s", target.user, target.host)
}

func encodePowershellCommand(script string) string {
	runes := []rune(script)
	utf16Data := utf16.Encode(runes)
	buf := make([]byte, 0, len(utf16Data)*2)
	for _, v := range utf16Data {
		buf = append(buf, byte(v), byte(v>>8))
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func startDetachedProcess(binary string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Process.Release()
}

func stopRemoteService(ctx context.Context, target sshTarget) error {
	script := "Stop-Process -Name xp2p -Force -ErrorAction SilentlyContinue"
	_, err := sshInvokePowershell(ctx, target, script)
	return err
}
