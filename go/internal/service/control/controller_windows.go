//go:build windows

package control

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type windowsController struct{}

func defaultController() Controller {
	return windowsController{}
}

func (windowsController) Start(ctx context.Context, role Role) error {
	name, err := serviceName(role)
	if err != nil {
		return err
	}
	return runSC(ctx, "start", name, allowServiceAlreadyRunning)
}

func (windowsController) Stop(ctx context.Context, role Role) error {
	name, err := serviceName(role)
	if err != nil {
		return err
	}
	return runSC(ctx, "stop", name, allowServiceNotStarted)
}

func (windowsController) Status(ctx context.Context, role Role) (Status, error) {
	name, err := serviceName(role)
	if err != nil {
		return Status{}, err
	}

	output, err := runSCOutput(ctx, "query", name, nil)
	if err != nil {
		return Status{}, err
	}

	state, active := parseServiceState(output)
	return Status{
		Active: active,
		State:  state,
		Detail: strings.TrimSpace(output),
	}, nil
}

type scExitHandler func(error) error

func allowServiceNotStarted(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "1062") {
		return nil
	}
	return err
}

func allowServiceAlreadyRunning(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "1056") {
		return nil
	}
	return err
}

func serviceName(role Role) (string, error) {
	switch role {
	case RoleClient:
		return "xp2p-client", nil
	case RoleServer:
		return "xp2p-server", nil
	default:
		return "", fmt.Errorf("unsupported role %q", role)
	}
}

func runSC(ctx context.Context, command string, service string, handler scExitHandler) error {
	_, err := runSCOutput(ctx, command, service, handler)
	return err
}

func runSCOutput(ctx context.Context, command, service string, handler scExitHandler) (string, error) {
	cmd := exec.CommandContext(ctx, "sc.exe", command, service)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if handler != nil {
		err = handler(err)
	}
	if err != nil {
		return "", fmt.Errorf("sc %s %s: %w (output: %s)", command, service, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func parseServiceState(output string) (string, bool) {
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
		return state, state == "RUNNING"
	}
	return "UNKNOWN", false
}
