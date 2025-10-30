package ping

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

// Options describes how Ping should behave.
type Options struct {
	Count   int
	Timeout int // seconds
}

// Run executes a platform specific ping command against the provided target.
func Run(ctx context.Context, target string, opts Options) error {
	if target == "" {
		return errors.New("ping target is required")
	}

	count := opts.Count
	if count <= 0 {
		count = 4
	}

	cmd, err := buildCommand(ctx, target, count, opts.Timeout)
	if err != nil {
		return err
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func buildCommand(ctx context.Context, target string, count, timeout int) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "windows":
		args := []string{"-n", strconv.Itoa(count), target}
		if timeout > 0 {
			args = append(args, "-w", strconv.Itoa(timeout*1000))
		}
		return exec.CommandContext(ctx, "ping", args...), nil
	case "darwin", "linux":
		args := []string{"-c", strconv.Itoa(count)}
		if timeout > 0 {
			args = append(args, "-W", strconv.Itoa(timeout))
		}
		args = append(args, target)
		return exec.CommandContext(ctx, "ping", args...), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
