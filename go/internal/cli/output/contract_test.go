package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestWriteErrorProducesSingleDocument(t *testing.T) {
	var out bytes.Buffer
	if err := WriteError(&out, "xp2p client list", "command_failed", errors.New("failed")); err != nil {
		t.Fatal(err)
	}
	var envelope ErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.SchemaVersion != SchemaVersion || envelope.Error.Code != "command_failed" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if out.Bytes()[out.Len()-1] != '\n' {
		t.Fatal("JSON document must end with a newline")
	}
}

func TestWriteErrorRedactsCredentials(t *testing.T) {
	var out bytes.Buffer
	source := errors.New("password=hunter2 link trojan://secret@example.com:443")
	if err := WriteError(&out, "xp2p server user add", "command_failed", source); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("hunter2")) || bytes.Contains(out.Bytes(), []byte("secret@")) {
		t.Fatalf("credential leaked: %s", out.String())
	}
}

func TestWrapJSONContractMatrix(t *testing.T) {
	tests := []struct {
		name             string
		defaultOperation bool
		run              func(*cobra.Command) error
		wantError        bool
		wantCode         string
		checkResult      func(*testing.T, map[string]any)
	}{
		{
			name: "typed result preserves JSON types and empty collections",
			run: func(cmd *cobra.Command) error {
				fmt.Fprint(cmd.OutOrStdout(), "\x1b[31mlegacy output\x1b[0m")
				return SetResult(cmd, struct {
					Name    string   `json:"name"`
					Count   int      `json:"count"`
					Enabled bool     `json:"enabled"`
					Items   []string `json:"items"`
				}{Name: "Привет\n\t\u0001", Count: 7, Enabled: true, Items: []string{}})
			},
			checkResult: func(t *testing.T, result map[string]any) {
				if result["name"] != "Привет\n\t\u0001" || result["count"] != float64(7) || result["enabled"] != true {
					t.Fatalf("typed result changed: %#v", result)
				}
				if items, ok := result["items"].([]any); !ok || len(items) != 0 {
					t.Fatalf("empty collection is not []: %#v", result["items"])
				}
			},
		},
		{
			name:             "payload-free mutation",
			defaultOperation: true,
			run:              func(*cobra.Command) error { return nil },
			checkResult: func(t *testing.T, result map[string]any) {
				if result["status"] != "completed" {
					t.Fatalf("status=%#v", result["status"])
				}
			},
		},
		{
			name:      "missing typed payload",
			run:       func(*cobra.Command) error { return nil },
			wantError: true,
			wantCode:  "missing_json_result",
		},
		{
			name: "handler stderr and error become one document",
			run: func(cmd *cobra.Command) error {
				fmt.Fprintln(cmd.ErrOrStderr(), "\x1b[31mwarning\x1b[0m")
				fmt.Fprintln(cmd.OutOrStdout(), "progress")
				return errors.New("boom")
			},
			wantError: true,
			wantCode:  "command_failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "leaf", RunE: func(cmd *cobra.Command, _ []string) error {
				return tc.run(cmd)
			}}
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			Classify(cmd, ClassJSON, "")
			WrapJSON(cmd, func() bool { return true }, tc.defaultOperation)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			err := cmd.Execute()
			if tc.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				if stdout.Len() != 0 {
					t.Fatalf("stdout=%q", stdout.String())
				}
				var envelope ErrorEnvelope
				if decodeErr := json.Unmarshal(stderr.Bytes(), &envelope); decodeErr != nil {
					t.Fatalf("stderr is not one JSON document: %v; %q", decodeErr, stderr.String())
				}
				if envelope.Error.Code != tc.wantCode {
					t.Fatalf("code=%q", envelope.Error.Code)
				}
				if strings.Contains(stderr.String(), "\x1b[") || strings.Contains(stderr.String(), "warning") {
					t.Fatalf("stderr was polluted: %q", stderr.String())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if stderr.Len() != 0 || strings.Contains(stdout.String(), "\x1b[") || strings.Contains(stdout.String(), "legacy output") {
				t.Fatalf("polluted streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			var raw struct {
				Result map[string]any `json:"result"`
			}
			if decodeErr := json.Unmarshal(stdout.Bytes(), &raw); decodeErr != nil {
				t.Fatalf("stdout is not one JSON document: %v; %q", decodeErr, stdout.String())
			}
			tc.checkResult(t, raw.Result)
		})
	}
}
