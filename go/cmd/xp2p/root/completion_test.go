package root

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestCompletionBashOutputIsNotPollutedByLogs(t *testing.T) {
	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"completion", "bash"})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("completion bash failed: %v", err)
	}
	if !strings.HasPrefix(output, "# bash completion for xp2p") {
		t.Fatalf("unexpected completion prefix: %q", firstLine(output))
	}
	if strings.Contains(output, "xp2p starting") {
		t.Fatalf("completion output contains startup log")
	}
}

func TestShellCompletionRequestOutputIsNotPollutedByLogs(t *testing.T) {
	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"__complete", "server", ""})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("__complete failed: %v", err)
	}
	if strings.Contains(output, "xp2p starting") {
		t.Fatalf("__complete output contains startup log")
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&buf, reader)
		done <- copyErr
	}()

	runErr := fn()
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close stdout writer: %v", closeErr)
	}
	if copyErr := <-done; copyErr != nil {
		t.Fatalf("read captured stdout: %v", copyErr)
	}
	if closeErr := reader.Close(); closeErr != nil {
		t.Fatalf("close stdout reader: %v", closeErr)
	}

	return buf.String(), runErr
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(value, "\n")
	return line
}
