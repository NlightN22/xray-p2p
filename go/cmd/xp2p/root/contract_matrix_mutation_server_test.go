package root

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func registerServerMutationContracts(registry map[string]mutationContract) {
	type definition struct {
		path        string
		successArgs []string
		failureArgs []string
		fixture     string
		sensitive   []string
	}
	base := serverMutationBase(false, false)
	controlPassword := "control-secret\n\t\x01Ω"
	disabledUser := serverMutationBase(true, false)
	disabledReverse := serverMutationBase(false, true)
	withSecondUser := strings.Replace(base, `user_id = "matrix-user"`, `user_id = "keep-user"`, 1) + `
[[server.trojan_users]]
email = "keep-user"
password = "keep-server-value"
`
	withForward := base + `
[[server.forward_rules]]
listen_address = "127.0.0.1"
listen_port = 62001
target_host = "remove.example"
target_port = 443
protocol = "tcp"
tag = "fwd-62001"
remark = "remove.example:443"
`
	withRedirect := base + serverRedirectFixture(false)
	withDisabledRedirect := base + serverRedirectFixture(true)
	definitions := []definition{
		{
			path:        "xp2p server forward add",
			successArgs: []string{"server", "forward", "add", "--target", "unicode-例.example:443", "--listen-port", "62101", "--proto", "tcp"},
			failureArgs: []string{"server", "forward", "add", "--target", "invalid-target"},
			fixture:     base,
		},
		{
			path:        "xp2p server forward remove",
			successArgs: []string{"server", "forward", "remove", "--listen-port", "62001"},
			failureArgs: []string{"server", "forward", "remove", "--listen-port", "62999"},
			fixture:     withForward,
		},
		{
			path:        "xp2p server redirect add",
			successArgs: []string{"server", "redirect", "add", "--domain", "unicode-例.example", "--tag", "reverse-a"},
			failureArgs: []string{"server", "redirect", "add", "--cidr", "not-a-cidr", "--tag", "reverse-a"},
			fixture:     base,
		},
		{
			path:        "xp2p server redirect remove",
			successArgs: []string{"server", "redirect", "remove", "--domain", "remove.example", "--tag", "reverse-a"},
			failureArgs: []string{"server", "redirect", "remove", "--domain", "missing.example", "--tag", "reverse-a"},
			fixture:     withRedirect,
		},
		{
			path:        "xp2p server redirect disable",
			successArgs: []string{"server", "redirect", "disable", "--domain", "remove.example", "--tag", "reverse-a"},
			failureArgs: []string{"server", "redirect", "disable", "--domain", "missing.example", "--tag", "reverse-a"},
			fixture:     withRedirect,
		},
		{
			path:        "xp2p server redirect enable",
			successArgs: []string{"server", "redirect", "enable", "--domain", "remove.example", "--tag", "reverse-a"},
			failureArgs: []string{"server", "redirect", "enable", "--domain", "missing.example", "--tag", "reverse-a"},
			fixture:     withDisabledRedirect,
		},
		{
			path:        "xp2p server reverse disable",
			successArgs: []string{"server", "reverse", "disable", "reverse-a"},
			failureArgs: []string{"server", "reverse", "disable", "missing"},
			fixture:     base,
		},
		{
			path:        "xp2p server reverse enable",
			successArgs: []string{"server", "reverse", "enable", "reverse-a"},
			failureArgs: []string{"server", "reverse", "enable", "missing"},
			fixture:     disabledReverse,
		},
		{
			path:        "xp2p server user disable",
			successArgs: []string{"server", "user", "disable", "matrix-user"},
			failureArgs: []string{"server", "user", "disable", "missing"},
			fixture:     base,
			sensitive:   []string{"initial-server-value"},
		},
		{
			path:        "xp2p server user enable",
			successArgs: []string{"server", "user", "enable", "matrix-user"},
			failureArgs: []string{"server", "user", "enable", "missing"},
			fixture:     disabledUser,
			sensitive:   []string{"initial-server-value"},
		},
		{
			path:        "xp2p server user update",
			successArgs: []string{"server", "user", "update", "matrix-user", "--new-id", "unicode-例", "--password", "replacement-server-value"},
			failureArgs: []string{"server", "user", "update", "matrix-user", "--password", controlPassword},
			fixture:     base,
			sensitive:   []string{"initial-server-value", "replacement-server-value", "control-secret"},
		},
		{
			path:        "xp2p server user remove",
			successArgs: []string{"server", "user", "remove", "--id", "matrix-user"},
			failureArgs: []string{"server", "user", "remove", "--id", ""},
			fixture:     withSecondUser,
			sensitive:   []string{"initial-server-value"},
		},
	}
	for _, item := range definitions {
		item := item
		factory := func(t *testing.T) mutationFixture {
			return newServerMutationFixture(t, item.fixture, item.successArgs, item.failureArgs, item.sensitive)
		}
		registerMutation(registry, item.path, factory, factory)
	}
	modeFactory := func(t *testing.T) mutationFixture {
		return newServerModeMutationFixture(t)
	}
	registerMutation(registry, "xp2p server mode", modeFactory, modeFactory)
	registerServerRedirectAccessContracts(registry, withRedirect)
}

func newServerModeMutationFixture(t *testing.T) mutationFixture {
	t.Helper()
	restore := servicecontrol.SetDefaultForTesting(&contractServiceController{mode: "inactive"})
	t.Cleanup(restore)
	content := strings.Replace(serverMutationBase(false, false), "[server]\n", "[server]\ntun_enabled = true\n", 1)
	fixture := newServerMutationFixture(
		t,
		content,
		[]string{"server", "mode", "proxy"},
		[]string{"server", "mode", "invalid"},
		nil,
	)
	desiredPath := filepath.Join(config.ConfigRoot(), layout.ServerConfigFileName)
	fixture.snapshot = func(t *testing.T) any {
		return snapshotModeMutation(t, desiredPath)
	}
	fixture.assertSuccess = func(t *testing.T, before, after any) {
		t.Helper()
		beforeState := before.(modeMutationSnapshot)
		afterState := after.(modeMutationSnapshot)
		if beforeState.desired == afterState.desired || !afterState.applyExists {
			t.Fatalf("server mode state was not fully staged: before=%#v after=%#v", beforeState, afterState)
		}
	}
	return fixture
}

func registerServerRedirectAccessContracts(registry map[string]mutationContract, fixture string) {
	type definition struct {
		name        string
		successTail []string
		failureTail []string
	}
	definitions := []definition{
		{"set", []string{"--access", "restricted", "--allow-user", "unicode-例"}, []string{"--access", "invalid"}},
		{"add-user", []string{"--allow-user", "unicode-例"}, []string{"--allow-user", ""}},
		{"remove-user", []string{"--allow-user", "seed-user"}, []string{"--allow-user", "missing"}},
		{"add-group", []string{"--allow-group", "unicode-例"}, []string{"--allow-group", ""}},
		{"remove-group", []string{"--allow-group", "seed-group"}, []string{"--allow-group", "missing"}},
		{"clear", nil, []string{"--domain", "missing.example"}},
	}
	for _, item := range definitions {
		item := item
		path := "xp2p server redirect access " + item.name
		target := []string{"server", "redirect", "access", item.name, "--domain", "remove.example", "--tag", "reverse-a"}
		successArgs := append(append([]string{}, target...), item.successTail...)
		failureArgs := append(append([]string{}, target...), item.failureTail...)
		if item.name != "set" {
			failureArgs = []string{"server", "redirect", "access", item.name, "--domain", "missing.example", "--tag", "reverse-a"}
		}
		factory := func(t *testing.T) mutationFixture {
			return newServerMutationFixture(t, fixture, successArgs, failureArgs, nil)
		}
		registerMutation(registry, path, factory, factory)
	}
}

func serverMutationBase(userDisabled, reverseDisabled bool) string {
	userState, reverseState := "", ""
	if userDisabled {
		userState = "\ndisabled = true"
	}
	if reverseDisabled {
		reverseState = "\ndisabled = true"
	}
	return fmt.Sprintf(`[server]
install_dir = %q
host = "127.0.0.1"

[[server.trojan_users]]
email = "matrix-user"
password = "initial-server-value"%s

[server.reverse_channels.reverse-a]
domain = "reverse.example"
host = "127.0.0.1"
user_id = "matrix-user"
tag = "reverse-a"%s
`, safeServerInstallDir(), userState, reverseState)
}

func serverRedirectFixture(disabled bool) string {
	state := ""
	if disabled {
		state = "\ndisabled = true"
	}
	return `
[[server.server_redirects]]
domain = "remove.example"
outbound_tag = "reverse-a"
access = "restricted"
users = ["seed-user"]
groups = ["seed-group"]` + state + "\n"
}

func newServerMutationFixture(
	t *testing.T,
	content string,
	successArgs []string,
	failureArgs []string,
	sensitive []string,
) mutationFixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	path := filepath.Join(root, layout.ServerConfigFileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return mutationFixture{
		args: successArgs, failureArgs: failureArgs, sensitive: sensitive,
		snapshot: func(t *testing.T) any {
			t.Helper()
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		},
		assertSuccess: func(t *testing.T, before, after any) {
			t.Helper()
			if before == after || strings.TrimSpace(after.(string)) == "" {
				t.Fatal("server Desired document was not updated")
			}
		},
	}
}
