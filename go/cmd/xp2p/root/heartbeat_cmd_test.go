package root

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
)

func TestHeartbeatContractCommandPrintsCurrentGoContract(t *testing.T) {
	var output bytes.Buffer
	cmd := newHeartbeatCommand()
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"contract"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var got heartbeat.Contract
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, output.String())
	}
	want := heartbeat.CurrentContract()
	if got.Schema != want.Schema || got.Version != want.Version ||
		len(got.Statuses) != len(want.Statuses) ||
		len(got.Capabilities) != len(want.Capabilities) {
		t.Fatalf("output does not match current contract: got=%+v want=%+v", got, want)
	}
}
