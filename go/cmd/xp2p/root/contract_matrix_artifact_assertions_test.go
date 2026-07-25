package root

import (
	"archive/zip"
	"encoding/json"
	"os"
	"strings"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

func assertStage4ArchiveSuccess(t *testing.T, command string, args []string, path string) {
	t.Helper()
	result := assertStage4Success(t, command, executeContractCase(args, false))
	if result["path"] != path {
		t.Fatalf("artifact path: got %#v want %q", result["path"], path)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("artifact was not created: info=%v err=%v", info, err)
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("artifact is not a valid zip: %v", err)
	}
	_ = reader.Close()
}

func assertStage4Success(t *testing.T, command string, execution contractExecution) map[string]any {
	t.Helper()
	if execution.exitCode != 0 || execution.err != nil || execution.stderr != "" {
		t.Fatalf("exit=%d err=%v stdout=%q stderr=%q", execution.exitCode, execution.err, execution.stdout, execution.stderr)
	}
	document := assertJSONDocument(t, execution.stdout)
	var envelope struct {
		SchemaVersion string         `json:"schema_version"`
		Command       string         `json:"command"`
		Result        map[string]any `json:"result"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != clioutput.SchemaVersion || envelope.Command != command {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if strings.Contains(execution.stdout, "\x1b[") {
		t.Fatalf("ANSI leaked to JSON output: %q", execution.stdout)
	}
	return envelope.Result
}

func assertStage4Failure(t *testing.T, command string, execution contractExecution, secrets ...string) {
	t.Helper()
	if execution.exitCode == 0 || execution.err == nil || execution.stdout != "" {
		t.Fatalf("invalid failure framing: exit=%d err=%v stdout=%q", execution.exitCode, execution.err, execution.stdout)
	}
	document := assertJSONDocument(t, execution.stderr)
	var envelope clioutput.ErrorEnvelope
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Command != command || envelope.Error.Code != "command_failed" {
		t.Fatalf("unexpected error envelope: %#v", envelope)
	}
	for _, secret := range secrets {
		if strings.Contains(execution.stderr, secret) {
			t.Fatalf("failure leaked secret %q: %q", secret, execution.stderr)
		}
	}
	if strings.Contains(execution.stderr, "\x1b[") {
		t.Fatalf("ANSI leaked to stderr: %q", execution.stderr)
	}
}

func assertStage4Human(t *testing.T, stdout, stderr string, err error, expected ...string) {
	t.Helper()
	if err != nil {
		t.Fatalf("human execution failed: %v stdout=%q stderr=%q", err, stdout, stderr)
	}
	raw := stdout + stderr
	if strings.Contains(raw, "\x1b[") {
		t.Fatalf("human output contains ANSI: %q", raw)
	}
	for _, value := range expected {
		if !strings.Contains(raw, value) {
			t.Fatalf("human output is missing %q: %q", value, raw)
		}
	}
}
