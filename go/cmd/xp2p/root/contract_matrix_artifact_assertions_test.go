package root

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
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

func assertStage4ArchiveMetadata(t *testing.T, path string, marker []byte) {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		stream, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		content, readErr := io.ReadAll(stream)
		_ = stream.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if bytes.Contains(content, marker) {
			return
		}
	}
	t.Fatalf("archive %s does not retain Unicode/control metadata %q", path, marker)
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
	assertExactStage4ResultSchema(t, command, envelope.Result)
	if strings.Contains(execution.stdout, "\x1b[") {
		t.Fatalf("ANSI leaked to JSON output: %q", execution.stdout)
	}
	return envelope.Result
}

type stage4SchemaField struct {
	kind   string
	nested map[string]stage4SchemaField
}

var stage4ResultSchemas = map[string]map[string]stage4SchemaField{
	"xp2p client debug bundle": {"path": {kind: "string"}},
	"xp2p client deploy": {
		"status": {kind: "string"}, "link": {kind: "string"},
		"install_dir": {kind: "string"}, "config_dir": {kind: "string"},
		"tun_enabled": {kind: "bool"}, "tun_mode": {kind: "string"},
		"service_active": {kind: "bool"},
	},
	"xp2p client export": {"path": {kind: "string"}},
	"xp2p client import": {
		"status": {kind: "string"}, "path": {kind: "string"},
	},
	"xp2p client install": {
		"status": {kind: "string"}, "install_dir": {kind: "string"},
		"config_dir": {kind: "string"}, "host": {kind: "string"},
		"port": {kind: "string"}, "user": {kind: "string"},
	},
	"xp2p server debug bundle": {"path": {kind: "string"}},
	"xp2p server export":       {"path": {kind: "string"}},
	"xp2p server identity provision": {
		"user_id": {kind: "string"}, "link": {kind: "string"},
	},
	"xp2p server import": {
		"status": {kind: "string"}, "path": {kind: "string"},
	},
	"xp2p server install": {
		"status": {kind: "string"}, "install_dir": {kind: "string"},
		"config_dir": {kind: "string"},
		"credential": {
			kind: "object",
			nested: map[string]stage4SchemaField{
				"user": {kind: "string"}, "password": {kind: "string"},
				"link": {kind: "string"},
			},
		},
		"warnings": {kind: "array"},
	},
	"xp2p server user add": {
		"user_id": {kind: "string"}, "password": {kind: "string"},
		"link": {kind: "string"}, "warnings": {kind: "array"},
	},
	"xp2p server user rotate": {
		"credential": {kind: "string"}, "generation": {kind: "number"},
		"previous_valid_until": {kind: "string"}, "user_id": {kind: "string"},
	},
}

func assertExactStage4ResultSchema(t *testing.T, command string, result map[string]any) {
	t.Helper()
	schema, ok := stage4ResultSchemas[command]
	if !ok {
		t.Fatalf("missing exact result schema for %s", command)
	}
	assertStage4ObjectSchema(t, "result", result, schema)
}

func assertStage4ObjectSchema(
	t *testing.T,
	path string,
	value map[string]any,
	schema map[string]stage4SchemaField,
) {
	t.Helper()
	if len(value) != len(schema) {
		t.Fatalf("%s keys changed: got=%v want=%v", path, sortedStage4Keys(value), sortedStage4SchemaKeys(schema))
	}
	for key, field := range schema {
		child, ok := value[key]
		if !ok {
			t.Fatalf("%s.%s is missing", path, key)
		}
		switch field.kind {
		case "string":
			if _, ok := child.(string); !ok {
				t.Fatalf("%s.%s type changed: %T", path, key, child)
			}
		case "bool":
			if _, ok := child.(bool); !ok {
				t.Fatalf("%s.%s type changed: %T", path, key, child)
			}
		case "number":
			if _, ok := child.(float64); !ok {
				t.Fatalf("%s.%s type changed: %T", path, key, child)
			}
		case "array":
			if _, ok := child.([]any); !ok {
				t.Fatalf("%s.%s type changed: %T", path, key, child)
			}
		case "object":
			object, ok := child.(map[string]any)
			if !ok {
				t.Fatalf("%s.%s type changed: %T", path, key, child)
			}
			assertStage4ObjectSchema(t, path+"."+key, object, field.nested)
		default:
			t.Fatalf("unsupported schema kind %q for %s.%s", field.kind, path, key)
		}
	}
}

func sortedStage4Keys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedStage4SchemaKeys(value map[string]stage4SchemaField) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

var (
	stage4UUIDPattern          = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	stage4TimePattern          = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z\b`)
	stage4LinkPattern          = regexp.MustCompile(`(?i)\b(?:trojan|vless)://\S+`)
	stage4GeneratedUserPattern = regexp.MustCompile(`\bclient-[a-z0-9]+@xp2p\.local\b`)
)

var stage4HumanBaselineDigests = map[string]string{
	"xp2p client debug bundle":       "f094f60bd65131e7d5b6690b802ab328096302d74437603ae62dae0ad8ae10ae",
	"xp2p client deploy":             "ab0610813ed654a35355964ca79553be9fafcb9e5d00222e6f142ebe2e7f7cdd",
	"xp2p client export":             "4cb22e4a28b058895bf32741b0810fab3235a2be3b6fe44c91152268ed931d7e",
	"xp2p client import":             "7c81e9094b712ba487111370ad6e3d9dd7af4eb4609dae57f43441c78fb8bc5c",
	"xp2p client install":            "ee922dd3531a9551fe35ec3a2e10a07b2f49f9fe4ce9c00df9f01e52a9c0b992",
	"xp2p server debug bundle":       "889167aa205e84dcbc2df66ebf2f05c27a505102d15db6a67fa10f0a74566660",
	"xp2p server export":             "0ce0e8162dda4e8b98811c30a796879b21d3584dc63cd534894a06dcb9b672f2",
	"xp2p server identity provision": "6e706c925c34cb57be02578c0055bc43e7b9c76ebd87657ff92f41f6e73957bf",
	"xp2p server import":             "9df5d604d6c23ca4c0b31c5863c22b58a97eaf83a049285328f7fac4e8ce3231",
	"xp2p server install":            "0564537466da2e70acfb7eaa168f54216177ed28cb69ad753a59e285ff5297b7",
	"xp2p server user add":           "1872f229e489f61324e4b1d69d23993d949681409ee849edb20d47eddedbf2ea",
	"xp2p server user rotate":        "ae4c93e6a801b67c0eff3ab240c6e9d176e2f523c1f33a8cee2e45243e319f0e",
}

func assertStage4Human(
	t *testing.T,
	command, stdout, stderr string,
	err error,
	expected ...string,
) {
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
	normalized := normalizeStage4Human(stdout) + normalizeStage4Human(stderr)
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(normalized)))
	want, ok := stage4HumanBaselineDigests[command]
	if !ok {
		t.Fatalf(
			"missing exact stage 4 human baseline for %s: digest=%s normalized=%q",
			command,
			digest,
			normalized,
		)
	}
	if digest != want {
		t.Fatalf(
			"stage 4 human baseline changed for %s: got=%s want=%s normalized=%q",
			command,
			digest,
			want,
			normalized,
		)
	}
}

func normalizeStage4Human(value string) string {
	value = normalizeHumanOutput(value)
	value = strings.ReplaceAll(value, `\`, "/")
	value = stage4UUIDPattern.ReplaceAllString(value, "<CREDENTIAL>")
	value = stage4TimePattern.ReplaceAllString(value, "<TIME>")
	value = stage4GeneratedUserPattern.ReplaceAllString(value, "<USER>")
	return stage4LinkPattern.ReplaceAllString(value, "<LINK>")
}
