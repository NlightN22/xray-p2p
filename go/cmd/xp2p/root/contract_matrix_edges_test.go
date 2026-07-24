package root

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

func TestJSONDocumentFramingRejectsTrailingData(t *testing.T) {
	for _, raw := range []string{
		"{\"result\":{}}\n{\"result\":{}}\n",
		"{\"result\":{}}\ntrailing",
		"{\"result\":{}} \n",
		"{\"result\":{}}\n\n",
		"{\"result\":{}}",
	} {
		if err := validateJSONDocument([]byte(raw)); err == nil {
			t.Fatalf("accepted invalid framing: %q", raw)
		}
	}
}

func validateJSONDocument(raw []byte) error {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return errors.New("JSON document must end with one newline")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var document json.RawMessage
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	if suffix := raw[decoder.InputOffset():]; !bytes.Equal(suffix, []byte("\n")) {
		return fmt.Errorf("JSON document must be followed by exactly one newline, got %q", suffix)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON data: %w", err)
	}
	return nil
}

func TestContractHarnessSuppressesHandlerWarnings(t *testing.T) {
	leaf := &cobra.Command{
		Use: "warning",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "human progress")
			fmt.Fprintln(cmd.ErrOrStderr(), "\x1b[33mhandler warning\x1b[0m")
			return clioutput.SetResult(cmd, struct {
				Count   int      `json:"count"`
				Enabled bool     `json:"enabled"`
				Items   []string `json:"items"`
				Label   string   `json:"label"`
			}{
				Count:   7,
				Enabled: true,
				Items:   []string{},
				Label:   "Miyuki\n\t\u0001",
			})
		},
	}
	clioutput.Classify(leaf, clioutput.ClassJSON, "")
	clioutput.WrapJSON(leaf, func() bool { return true })

	var stdout, stderr bytes.Buffer
	leaf.SetOut(&stdout)
	leaf.SetErr(&stderr)
	if err := leaf.Execute(); err != nil {
		t.Fatal(err)
	}
	assertJSONDocument(t, stdout.String())
	if stderr.Len() != 0 || strings.Contains(stdout.String(), "human progress") ||
		strings.Contains(stdout.String(), "handler warning") ||
		strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("polluted streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func captureProcessStreams(execute func() error) (string, string, error) {
	originalStdout, originalStderr := os.Stdout, os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return "", "", err
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return "", "", err
	}
	var stdout, stderr bytes.Buffer
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stdout, stdoutReader)
		close(stdoutDone)
	}()
	go func() {
		_, _ = io.Copy(&stderr, stderrReader)
		close(stderrDone)
	}()
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	runErr := execute()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout, os.Stderr = originalStdout, originalStderr
	<-stdoutDone
	<-stderrDone
	_ = stdoutReader.Close()
	_ = stderrReader.Close()
	return stdout.String(), stderr.String(), runErr
}

func TestContractHarnessSuppressesPromptInClientRemoveHandler(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", configRoot)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(configRoot, "logs"))
	installDir := filepath.Join(configRoot, "install")
	args := []string{
		"--json", "client", "remove", "--all", "--keep-files", "--ignore-missing",
		"--path", installDir, "--config-dir", "client",
	}
	root := NewCommandForArgs(args)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v; stderr=%q", err, stderr.String())
	}
	document := assertJSONDocument(t, stdout.String())
	var envelope struct {
		Result mutationResult `json:"result"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Result.Status != "completed" || envelope.Result.Operation != "client remove" {
		t.Fatalf("real client remove handler did not complete: %#v", envelope.Result)
	}
	if stderr.Len() != 0 || strings.Contains(stdout.String(), "Confirm removal?") {
		t.Fatalf("prompt polluted JSON streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
