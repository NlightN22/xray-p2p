//go:build windows

package scctl

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

func Run(ctx context.Context, command, service string, handler func(error) error) error {
	_, err := RunOutput(ctx, command, service, handler)
	return err
}

func RunOutput(ctx context.Context, command, service string, handler func(error) error) (string, error) {
	scPath, err := resolveSCPath()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, scPath, command, service)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if handler != nil {
		err = handler(err)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("sc %s %s: %w (output: %s)", command, service, err, msg)
	}
	return stdout.String(), nil
}

func AllowServiceNotStarted(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "1062") {
		return nil
	}
	return err
}

func AllowServiceAlreadyRunning(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "1056") {
		return nil
	}
	return err
}

func ParseServiceState(output string) (svc.State, bool) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "STATE") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		section := strings.TrimSpace(parts[1])
		fields := strings.Fields(section)
		if len(fields) == 0 {
			continue
		}
		state := strings.ToUpper(fields[len(fields)-1])
		switch state {
		case "RUNNING":
			return svc.Running, true
		case "STOPPED":
			return svc.Stopped, false
		case "START_PENDING":
			return svc.StartPending, false
		case "STOP_PENDING":
			return svc.StopPending, false
		case "CONTINUE_PENDING":
			return svc.ContinuePending, false
		case "PAUSE_PENDING":
			return svc.PausePending, false
		case "PAUSED":
			return svc.Paused, false
		default:
			return svc.Unknown, false
		}
	}
	return svc.Unknown, false
}

func IsServiceMissing(err error) bool {
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "1060") || strings.Contains(msg, "does not exist")
}

func resolveSCPath() (string, error) {
	if path, err := exec.LookPath("sc.exe"); err == nil {
		return path, nil
	}
	roots := []string{
		os.Getenv("SystemRoot"),
		os.Getenv("WINDIR"),
	}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		candidates := []string{
			filepath.Join(root, "System32", "sc.exe"),
		}
		if os.Getenv("PROCESSOR_ARCHITEW6432") != "" {
			candidates = append(candidates, filepath.Join(root, "Sysnative", "sc.exe"))
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("sc.exe not found")
}
