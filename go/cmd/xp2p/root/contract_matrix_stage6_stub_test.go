//go:build !linux

package root

import (
	"encoding/json"
	"strings"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

func registerStage6PlatformContractCases(map[string]contractCase) {}

func TestStage6UnsupportedPlatformContracts(t *testing.T) {
	leaves := jsonLeafPaths(NewCommand())
	argsByPath := map[string][]string{
		"xp2p client dns-forward add":    {"client", "dns-forward", "add", "--domain", "example.test", "--target", "192.0.2.1:53", "--intercept", "--quiet", "--debug"},
		"xp2p client dns-forward list":   {"client", "dns-forward", "list", "--debug"},
		"xp2p client dns-forward remove": {"client", "dns-forward", "remove", "--domain", "example.test", "--with-forward", "--intercept", "--quiet", "--debug"},
		"xp2p server dns-forward add":    {"server", "dns-forward", "add", "--domain", "example.test", "--target", "192.0.2.1:53", "--intercept", "--quiet", "--debug"},
		"xp2p server dns-forward list":   {"server", "dns-forward", "list", "--debug"},
		"xp2p server dns-forward remove": {"server", "dns-forward", "remove", "--all", "--with-forward", "--intercept", "--quiet", "--debug"},
		"xp2p nat-redirect add":          {"nat-redirect", "add", "--cidr", "10.0.0.0/24", "--port", "12345", "--print-only", "--quiet", "--snippet", "test.nft", "--entry-dir", "entries", "--inbounds", "inbounds.json"},
		"xp2p nat-redirect list":         {"nat-redirect", "list"},
		"xp2p nat-redirect remove":       {"nat-redirect", "remove", "--all", "--print-only", "--snippet", "test.nft", "--entry-dir", "entries"},
	}
	for _, path := range stage6Paths {
		if !leaves[path] {
			t.Errorf("platform stub is absent from non-Linux Cobra tree: %s", path)
			continue
		}
		args, ok := argsByPath[path]
		if !ok {
			t.Fatalf("missing unsupported-platform invocation for %s", path)
		}
		execution := executeContractCase(args, false)
		if execution.exitCode == 0 || execution.err == nil || execution.stdout != "" {
			t.Fatalf("%s: exit=%d err=%v stdout=%q", path, execution.exitCode, execution.err, execution.stdout)
		}
		document := assertJSONDocument(t, execution.stderr)
		var envelope clioutput.ErrorEnvelope
		if err := json.Unmarshal(document, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Command != path || envelope.Error.Code != "unsupported_platform" ||
			!strings.Contains(envelope.Error.Message, "supported only") {
			t.Errorf("%s: envelope=%#v", path, envelope)
		}
		if strings.Contains(execution.stderr, "Usage:") ||
			strings.Contains(execution.stderr, "\x1b[") ||
			strings.Contains(execution.stderr, "level=") {
			t.Errorf("%s: error output contains usage, ANSI, or logs: %q", path, execution.stderr)
		}
	}
}
