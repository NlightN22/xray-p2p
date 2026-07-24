package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

func TestProcessEntryPointReturnsNonZeroJSONError(t *testing.T) {
	name := "xp2p-contract-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build process fixture: %v\n%s", err, output)
	}

	command := exec.Command(binary, "--json", "heartbeat", "contract", "extra")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() == 0 {
		t.Fatalf("process error=%T %v, want non-zero exit; stdout=%q stderr=%q", err, err, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("process stdout=%q, want empty", stdout.String())
	}
	var envelope clioutput.ErrorEnvelope
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("decode process stderr: %v; raw=%q", err, stderr.String())
	}
	if envelope.Command != "xp2p heartbeat contract" || envelope.Error.Code != "invalid_argument" {
		t.Fatalf("unexpected process error envelope: %#v", envelope)
	}
}
