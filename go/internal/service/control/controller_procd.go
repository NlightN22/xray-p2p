//go:build linux

package control

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type procdController struct{}

func (procdController) Start(ctx context.Context, role Role) error {
	script, err := initScriptPath(role)
	if err != nil {
		return err
	}
	return runInitScript(ctx, script, "start")
}

func (procdController) Stop(ctx context.Context, role Role) error {
	script, err := initScriptPath(role)
	if err != nil {
		return err
	}
	return runInitScript(ctx, script, "stop")
}

func (procdController) Status(ctx context.Context, role Role) (Status, error) {
	script, err := initScriptPath(role)
	if err != nil {
		return Status{}, err
	}

	state := "inactive"
	active := false
	runningErr := runInitScript(ctx, script, "running")
	if runningErr == nil {
		active = true
		state = "running"
	}

	detail, detailErr := runInitScriptOutput(ctx, script, "status")
	if detailErr != nil && active {
		return Status{}, fmt.Errorf("xp2p: procd status %s: %w", script, detailErr)
	}

	return Status{
		Active: active,
		State:  state,
		Detail: strings.TrimSpace(detail),
	}, nil
}

func initScriptPath(role Role) (string, error) {
	var script string
	switch role {
	case RoleClient:
		script = "/etc/init.d/xp2p-client"
	case RoleServer:
		script = "/etc/init.d/xp2p-server"
	default:
		return "", fmt.Errorf("unsupported role %q", role)
	}
	if _, err := os.Stat(script); err != nil {
		return "", fmt.Errorf("xp2p: init script %s not found: %w", script, err)
	}
	return script, nil
}

func runInitScript(ctx context.Context, script string, arg string) error {
	cmd := exec.CommandContext(ctx, script, arg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xp2p: %s %s failed: %w (output: %s)", script, arg, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runInitScriptOutput(ctx context.Context, script string, arg string) (string, error) {
	cmd := exec.CommandContext(ctx, script, arg)
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
