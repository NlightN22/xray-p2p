package root

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func registerHAMutationContracts(registry map[string]mutationContract) {
	type definition struct {
		path           string
		successArgs    []string
		successPrepare [][]string
		failureArgs    []string
		failurePrepare [][]string
		failureContent string
	}
	definitions := []definition{
		{
			path:           "xp2p server ha group create",
			successArgs:    []string{"server", "ha", "group", "create", "group-1", "edge-group"},
			failureArgs:    []string{"server", "ha", "group", "create", "group-2", "other-group"},
			failurePrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
		},
		{
			path:           "xp2p server ha group update",
			successArgs:    []string{"server", "ha", "group", "update", "manual"},
			successPrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
			failureArgs:    []string{"server", "ha", "group", "update", "manual"},
		},
		{
			path:           "xp2p server ha group remove",
			successArgs:    []string{"server", "ha", "group", "remove"},
			successPrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
			failureArgs:    []string{"server", "ha", "group", "remove"},
		},
		{
			path:        "xp2p server ha peer self",
			successArgs: []string{"server", "ha", "peer", "self", "node-a"},
			failureArgs: []string{"server", "ha", "peer", "self", ""},
		},
		{
			path:        "xp2p server ha peer add",
			successArgs: []string{"server", "ha", "peer", "add", "node-b", "https://127.0.0.1:9443", "peer-auth-value", "--allow-insecure"},
			failureArgs: []string{"server", "ha", "peer", "add", "", "not-an-endpoint", "peer-auth-value"},
		},
		{
			path:           "xp2p server ha peer remove",
			successArgs:    []string{"server", "ha", "peer", "remove", "node-b"},
			successPrepare: [][]string{{"server", "ha", "peer", "add", "node-b", "https://127.0.0.1:9443", "peer-auth-value", "--allow-insecure"}},
			failureArgs:    []string{"server", "ha", "peer", "remove", ""},
			failureContent: "[server.ha\n",
		},
		{
			path:           "xp2p server ha channel create",
			successArgs:    []string{"server", "ha", "channel", "create", "channel-1", "reverse-a", "matrix.example"},
			successPrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
			failureArgs:    []string{"server", "ha", "channel", "create", "channel-1", "reverse-a", "matrix.example"},
		},
		{
			path:           "xp2p server ha channel disable",
			successArgs:    []string{"server", "ha", "channel", "disable", "channel-1"},
			successPrepare: haChannelWithMemberPrepare(),
			failureArgs:    []string{"server", "ha", "channel", "disable", "missing"},
			failurePrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
		},
		{
			path:           "xp2p server ha channel rebind",
			successArgs:    []string{"server", "ha", "channel", "rebind", "channel-1", "endpoint-a"},
			successPrepare: haChannelWithMemberPrepare(),
			failureArgs:    []string{"server", "ha", "channel", "rebind", "missing", "endpoint-a"},
			failurePrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
		},
		{
			path:           "xp2p server ha channel rebind-endpoint",
			successArgs:    []string{"server", "ha", "channel", "rebind-endpoint", "channel-1", "endpoint-a"},
			successPrepare: haChannelWithMemberPrepare(),
			failureArgs:    []string{"server", "ha", "channel", "rebind-endpoint", "missing", "endpoint-a"},
			failurePrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
		},
		{
			path:           "xp2p server ha channel finalize",
			successArgs:    []string{"server", "ha", "channel", "finalize", "channel-1"},
			successPrepare: append(haChannelPrepare(), []string{"server", "ha", "channel", "disable", "channel-1"}),
			failureArgs:    []string{"server", "ha", "channel", "finalize", "channel-1"},
			failurePrepare: haChannelPrepare(),
		},
		{
			path:           "xp2p server ha member add",
			successArgs:    []string{"server", "ha", "member", "add", "member-1", "endpoint-a", "203.0.113.10", "443", "trojan-tls"},
			successPrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
			failureArgs:    []string{"server", "ha", "member", "add", "member-1", "endpoint-a", "203.0.113.10", "bad-port", "trojan-tls"},
			failurePrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
		},
		{
			path:           "xp2p server ha member reprioritize",
			successArgs:    []string{"server", "ha", "member", "reprioritize", "member-1", "20"},
			successPrepare: haMemberPrepare(),
			failureArgs:    []string{"server", "ha", "member", "reprioritize", "missing", "20"},
			failurePrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
		},
		{
			path:           "xp2p server ha member remove",
			successArgs:    []string{"server", "ha", "member", "remove", "member-1"},
			successPrepare: haMemberPrepare(),
			failureArgs:    []string{"server", "ha", "member", "remove", "missing"},
			failurePrepare: [][]string{{"server", "ha", "group", "create", "group-1", "edge-group"}},
		},
		{
			path:           "xp2p server ha redirect add",
			successArgs:    []string{"server", "ha", "redirect", "add", "channel-1", "--domain", "unicode-例.example"},
			successPrepare: haChannelPrepare(),
			failureArgs:    []string{"server", "ha", "redirect", "add", "missing", "--domain", "matrix.example"},
			failurePrepare: haChannelPrepare(),
		},
		{
			path:           "xp2p server ha redirect remove",
			successArgs:    []string{"server", "ha", "redirect", "remove", "channel-1", "--domain", "matrix.example"},
			successPrepare: append(haChannelPrepare(), []string{"server", "ha", "redirect", "add", "channel-1", "--domain", "matrix.example"}),
			failureArgs:    []string{"server", "ha", "redirect", "remove", "channel-1", "--domain", "missing.example"},
			failurePrepare: haChannelPrepare(),
		},
		{
			path:        "xp2p server ha sync",
			successArgs: []string{"server", "ha", "sync"},
			successPrepare: [][]string{
				{"server", "ha", "group", "create", "group-1", "edge-group"},
				{"server", "ha", "peer", "self", "node-a"},
			},
			failureArgs: []string{"server", "ha", "sync"},
		},
	}
	for _, item := range definitions {
		item := item
		registerMutation(
			registry,
			item.path,
			func(t *testing.T) mutationFixture {
				return newHAMutationFixture(t, item.successArgs, item.failureArgs, item.successPrepare)
			},
			func(t *testing.T) mutationFixture {
				fixture := newHAMutationFixture(t, item.successArgs, item.failureArgs, item.failurePrepare)
				if item.failureContent != "" {
					path := filepath.Join(os.Getenv("XP2P_CONFIG_ROOT"), layout.ServerConfigFileName)
					if err := os.WriteFile(path, []byte(item.failureContent), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return fixture
			},
		)
	}
}

func haChannelPrepare() [][]string {
	return [][]string{
		{"server", "ha", "group", "create", "group-1", "edge-group"},
		{"server", "ha", "channel", "create", "channel-1", "reverse-a", "matrix.example"},
	}
}

func haMemberPrepare() [][]string {
	return [][]string{
		{"server", "ha", "group", "create", "group-1", "edge-group"},
		{"server", "ha", "member", "add", "member-1", "endpoint-a", "203.0.113.10", "443", "trojan-tls"},
	}
}

func haChannelWithMemberPrepare() [][]string {
	return append(haMemberPrepare(),
		[]string{"server", "ha", "channel", "create", "channel-1", "reverse-a", "matrix.example"})
}

func newHAMutationFixture(
	t *testing.T,
	successArgs []string,
	failureArgs []string,
	prepare [][]string,
) mutationFixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	for _, args := range prepare {
		mustRunMutationPrerequisite(t, args)
	}
	path := filepath.Join(root, layout.ServerConfigFileName)
	return mutationFixture{
		args:        successArgs,
		failureArgs: failureArgs,
		sensitive:   []string{"peer-auth-value"},
		snapshot: func(t *testing.T) any {
			t.Helper()
			data, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				return ""
			}
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		},
		assertSuccess: func(t *testing.T, before, after any) {
			t.Helper()
			if before == after {
				t.Fatal("HA Desired document was not updated")
			}
		},
	}
}

func mustRunMutationPrerequisite(t *testing.T, args []string) {
	t.Helper()
	execution := executeContractCase(args, false)
	if execution.exitCode != 0 || execution.err != nil {
		t.Fatalf("prepare %v: exit=%d err=%v stderr=%q", args, execution.exitCode, execution.err, execution.stderr)
	}
}
