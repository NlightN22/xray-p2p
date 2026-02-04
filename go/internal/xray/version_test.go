package xray

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestParseVersionOutput(t *testing.T) {
	version, err := parseVersionOutput("Xray 25.10.15 (Xray, Penetrates Everything.)")
	if err != nil {
		t.Fatalf("parseVersionOutput error: %v", err)
	}
	if version != "25.10.15" {
		t.Fatalf("unexpected version: %q", version)
	}
}

func TestParseVersionOutputInvalid(t *testing.T) {
	if _, err := parseVersionOutput("broken"); err == nil {
		t.Fatalf("expected error for invalid output")
	}
}

func TestVerifyPinnedVersionMismatchAllowed(t *testing.T) {
	orig := execCommand
	execCommand = fakeCommand("Xray 0.0.0 (test)")
	defer func() { execCommand = orig }()

	os.Setenv(envAllowMismatch, "1")
	t.Cleanup(func() { os.Unsetenv(envAllowMismatch) })

	if err := VerifyPinnedVersion(context.Background(), "xray"); err != nil {
		t.Fatalf("VerifyPinnedVersion error: %v", err)
	}
}

func TestVerifyPinnedVersionMismatchFails(t *testing.T) {
	orig := execCommand
	execCommand = fakeCommand("Xray 0.0.0 (test)")
	defer func() { execCommand = orig }()

	os.Unsetenv(envAllowMismatch)

	if err := VerifyPinnedVersion(context.Background(), "xray"); err == nil {
		t.Fatalf("expected mismatch error")
	}
}

func TestVerifyPinnedVersionMatch(t *testing.T) {
	pinned, err := PinnedVersion()
	if err != nil {
		t.Fatalf("PinnedVersion error: %v", err)
	}
	orig := execCommand
	execCommand = fakeCommand("Xray " + pinned + " (test)")
	defer func() { execCommand = orig }()

	if err := VerifyPinnedVersion(context.Background(), "xray"); err != nil {
		t.Fatalf("VerifyPinnedVersion error: %v", err)
	}
}

func TestVerifyPinnedVersionSkip(t *testing.T) {
	orig := execCommand
	execCommand = fakeCommand("broken")
	defer func() { execCommand = orig }()

	os.Setenv(envSkipCheck, "1")
	t.Cleanup(func() { os.Unsetenv(envSkipCheck) })

	if err := VerifyPinnedVersion(context.Background(), "xray"); err != nil {
		t.Fatalf("VerifyPinnedVersion error: %v", err)
	}
}

func fakeCommand(output string) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		args := []string{"-test.run=TestHelperProcess", "--", output}
		cmd := exec.CommandContext(ctx, os.Args[0], args...)
		cmd.Env = append(os.Environ(), "XP2P_TEST_HELPER=1")
		return cmd
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("XP2P_TEST_HELPER") != "1" {
		return
	}
	args := os.Args
	idx := 0
	for idx < len(args) && args[idx] != "--" {
		idx++
	}
	if idx+1 >= len(args) {
		os.Exit(1)
	}
	output := strings.Join(args[idx+1:], " ")
	os.Stdout.WriteString(output)
	os.Exit(0)
}
