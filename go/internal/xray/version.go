package xray

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const (
	envAllowMismatch = "XP2P_XRAY_ALLOW_MISMATCH"
	envSkipCheck     = "XP2P_XRAY_SKIP_VERSION_CHECK"
)

var execCommand = exec.CommandContext

// VerifyPinnedVersion checks xray --version output against the pinned version.
func VerifyPinnedVersion(ctx context.Context, xrayPath string) error {
	if isEnvTrue(envSkipCheck) {
		logging.Warn("xray version check skipped via env", "env", envSkipCheck)
		return nil
	}
	pinned, err := PinnedVersion()
	if err != nil {
		return err
	}
	actual, err := readVersion(ctx, xrayPath)
	if err != nil {
		return err
	}
	if actual == pinned {
		return nil
	}
	if isEnvTrue(envAllowMismatch) {
		logging.Warn("xray version mismatch allowed", "expected", pinned, "actual", actual)
		return nil
	}
	return fmt.Errorf("xray version mismatch (expected %s, got %s)", pinned, actual)
}

func readVersion(ctx context.Context, xrayPath string) (string, error) {
	cmd := execCommand(ctx, xrayPath, "-version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read xray version: %w", err)
	}
	return parseVersionOutput(string(out))
}

func parseVersionOutput(output string) (string, error) {
	fields := strings.Fields(output)
	if len(fields) < 2 {
		return "", errors.New("unexpected xray --version output")
	}
	version := strings.TrimSpace(fields[1])
	if version == "" {
		return "", errors.New("xray version is empty")
	}
	return version, nil
}

func isEnvTrue(name string) bool {
	val := strings.TrimSpace(os.Getenv(name))
	if val == "" {
		return false
	}
	switch strings.ToLower(val) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
