//go:build linux

package control

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type systemdController struct{}

func (systemdController) Start(ctx context.Context, role Role) error {
	unit := unitName(role)
	if unit == "" {
		return fmt.Errorf("unsupported role %q", role)
	}
	return runSystemctl(ctx, "start", unit)
}

func (systemdController) Stop(ctx context.Context, role Role) error {
	unit := unitName(role)
	if unit == "" {
		return fmt.Errorf("unsupported role %q", role)
	}
	return runSystemctl(ctx, "stop", unit)
}

func (systemdController) Status(ctx context.Context, role Role) (Status, error) {
	unit := unitName(role)
	if unit == "" {
		return Status{}, fmt.Errorf("unsupported role %q", role)
	}

	stateOut, stateErr := runSystemctlOutput(ctx, "is-active", unit)
	state := strings.TrimSpace(stateOut)
	active := state == "active"
	if stateErr != nil {
		if exitErr, ok := stateErr.(*exec.ExitError); ok {
			// systemctl is-active returns 3 when inactive.
			if exitErr.ExitCode() != 3 {
				return Status{}, fmt.Errorf("systemctl is-active %s: %w", unit, stateErr)
			}
		} else {
			return Status{}, fmt.Errorf("systemctl is-active %s: %w", unit, stateErr)
		}
	}

	detailOut, detailErr := runSystemctlOutput(ctx, "status", "--no-pager", "--lines=0", unit)
	if detailErr != nil {
		if exitErr, ok := detailErr.(*exec.ExitError); ok {
			switch exitErr.ExitCode() {
			case 3, 4:
				// inactive or unknown; keep detail output if any
			default:
				return Status{}, fmt.Errorf("systemctl status %s: %w", unit, detailErr)
			}
		} else {
			return Status{}, fmt.Errorf("systemctl status %s: %w", unit, detailErr)
		}
	}

	return Status{
		Active: active,
		State:  state,
		Detail: strings.TrimSpace(detailOut),
	}, nil
}

func runSystemctl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %w (output: %s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runSystemctlOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func unitName(role Role) string {
	switch role {
	case RoleClient:
		return "xp2p-client.service"
	case RoleServer:
		return "xp2p-server.service"
	default:
		return ""
	}
}
