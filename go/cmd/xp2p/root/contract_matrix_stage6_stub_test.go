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
	for _, path := range stage6Paths {
		if !leaves[path] {
			t.Errorf("platform stub is absent from non-Linux Cobra tree: %s", path)
			continue
		}
		args := strings.Fields(strings.TrimPrefix(path, "xp2p "))
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
	}
}
