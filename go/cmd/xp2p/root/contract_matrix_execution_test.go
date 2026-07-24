package root

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type contractExecution struct {
	stdout   string
	stderr   string
	err      error
	exitCode int
}

func executeContractCase(args []string, emitWarning bool) contractExecution {
	allArgs := append([]string{"--json"}, args...)
	cmd := NewCommandForArgs(allArgs)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(allArgs)
	if emitWarning {
		target, _, err := cmd.Find(args)
		if err != nil {
			return contractExecution{err: err, exitCode: ProcessExitCode(err)}
		}
		original := target.RunE
		target.RunE = func(cmd *cobra.Command, args []string) error {
			logging.Warn("matrix warning \x1b[31m token=matrix-secret\nnext")
			return original(cmd, args)
		}
	}
	processStdout, processStderr, err := captureProcessStreams(cmd.Execute)
	return contractExecution{
		stdout: stdout.String() + processStdout,
		stderr: stderr.String() + processStderr,
		err:    err, exitCode: ProcessExitCode(err),
	}
}

func assertReadOnlyEdgeCases(t *testing.T, result map[string]any, stdout, stderr string) {
	t.Helper()
	if stderr != "" {
		t.Fatalf("JSON diagnostic stream is not empty: %q", stderr)
	}
	raw := stdout + fmt.Sprintf("%v", result)
	for _, forbidden := range []string{"\x1b[", "matrix-secret", "PRIVATE KEY", "password:", "token="} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("executable edge check found %q in JSON output: %q", forbidden, stdout)
		}
	}
	assertNoCredentialFields(t, result, "result")
}

func assertNoCredentialFields(t *testing.T, value any, path string) {
	t.Helper()
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			normalized := strings.ToLower(key)
			for _, forbidden := range []string{"password", "secret", "token", "private_key", "credential"} {
				if strings.Contains(normalized, forbidden) {
					t.Fatalf("credential-shaped field %s.%s is present", path, key)
				}
			}
			assertNoCredentialFields(t, child, path+"."+key)
		}
	case []any:
		for index, child := range item {
			assertNoCredentialFields(t, child, fmt.Sprintf("%s[%d]", path, index))
		}
	case string:
		if strings.Contains(item, "trojan://") || strings.Contains(item, "vless://") {
			t.Fatalf("credential link is present at %s", path)
		}
	}
}
