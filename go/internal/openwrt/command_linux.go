//go:build linux

package openwrt

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func runCommand(name string, args ...string) error {
	return runCommandContext(context.Background(), name, args...)
}

func runCommandContext(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 2 * time.Second
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), ctxErr)
		}
		return fmt.Errorf("%s %s: %v (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(buf.String()))
	}
	return nil
}

func captureCommand(name string, args ...string) (string, error) {
	return captureCommandContext(context.Background(), name, args...)
}

func captureCommandContext(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 2 * time.Second
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		err = fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), ctxErr)
	}
	return strings.TrimSpace(buf.String()), err
}
