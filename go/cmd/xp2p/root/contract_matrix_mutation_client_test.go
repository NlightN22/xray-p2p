package root

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func registerClientMutationContracts(registry map[string]mutationContract) {
	type definition struct {
		path        string
		successArgs []string
		failureArgs []string
		fixture     string
		sensitive   []string
	}
	base := clientMutationBase(false)
	controlPassword := "control-secret\n\t\x01Ω"
	disabled := clientMutationBase(true)
	withForward := base + `
[[client.forwards]]
listen_address = "127.0.0.1"
listen_port = 62001
target_host = "remove.example"
target_port = 443
protocol = "tcp"
tag = "fwd-62001"
remark = "remove.example:443"
`
	withRedirect := base + `
[[client.redirects]]
domain = "remove.example"
outbound_tag = "edge-a"
`
	definitions := []definition{
		{
			path:        "xp2p client disable",
			successArgs: []string{"client", "disable", "edge-a"},
			failureArgs: []string{"client", "disable", "missing"},
			fixture:     base,
		},
		{
			path:        "xp2p client enable",
			successArgs: []string{"client", "enable", "edge-a"},
			failureArgs: []string{"client", "enable", "missing"},
			fixture:     disabled,
		},
		{
			path:        "xp2p client update",
			successArgs: []string{"client", "update", "edge-a", "--user", "matrix-user", "--password", controlPassword},
			failureArgs: []string{"client", "update", "missing", "--user", "matrix-user"},
			fixture:     base,
			sensitive:   []string{"control-secret", "initial-value"},
		},
		{
			path:        "xp2p client forward add",
			successArgs: []string{"client", "forward", "add", "--target", "unicode-例.example:443", "--listen-port", "62101", "--proto", "tcp"},
			failureArgs: []string{"client", "forward", "add", "--target", "invalid-target"},
			fixture:     base,
		},
		{
			path:        "xp2p client forward remove",
			successArgs: []string{"client", "forward", "remove", "--listen-port", "62001"},
			failureArgs: []string{"client", "forward", "remove", "--listen-port", "62999"},
			fixture:     withForward,
		},
		{
			path:        "xp2p client redirect add",
			successArgs: []string{"client", "redirect", "add", "--domain", "unicode-例.example", "--tag", "edge-a"},
			failureArgs: []string{"client", "redirect", "add", "--cidr", "not-a-cidr", "--tag", "edge-a"},
			fixture:     base,
		},
		{
			path:        "xp2p client redirect remove",
			successArgs: []string{"client", "redirect", "remove", "--domain", "remove.example", "--tag", "edge-a"},
			failureArgs: []string{"client", "redirect", "remove", "--domain", "missing.example", "--tag", "edge-a"},
			fixture:     withRedirect,
		},
		{
			path:        "xp2p client redirect disable",
			successArgs: []string{"client", "redirect", "disable", "--domain", "remove.example", "--tag", "edge-a"},
			failureArgs: []string{"client", "redirect", "disable", "--domain", "missing.example", "--tag", "edge-a"},
			fixture:     withRedirect,
		},
		{
			path:        "xp2p client redirect enable",
			successArgs: []string{"client", "redirect", "enable", "--domain", "remove.example", "--tag", "edge-a"},
			failureArgs: []string{"client", "redirect", "enable", "--domain", "missing.example", "--tag", "edge-a"},
			fixture:     strings.Replace(withRedirect, "outbound_tag = \"edge-a\"\n", "outbound_tag = \"edge-a\"\ndisabled = true\n", 1),
		},
		{
			path:        "xp2p client reverse disable",
			successArgs: []string{"client", "reverse", "disable", "reverse-a"},
			failureArgs: []string{"client", "reverse", "disable", "missing"},
			fixture:     base,
		},
		{
			path:        "xp2p client reverse enable",
			successArgs: []string{"client", "reverse", "enable", "reverse-a"},
			failureArgs: []string{"client", "reverse", "enable", "missing"},
			fixture:     disabledClientReverseFixture(),
		},
	}
	for _, item := range definitions {
		item := item
		factory := func(t *testing.T) mutationFixture {
			fixture := newClientMutationFixture(t, item.fixture, item.successArgs, item.failureArgs, item.sensitive)
			if item.path == "xp2p client update" {
				assertDesired := fixture.assertSuccess
				fixture.assertSuccess = func(t *testing.T, before, after any) {
					t.Helper()
					assertDesired(t, before, after)
					document := readPersistedCredentials(t, config.ConfigPath(layout.ClientConfigFileName))
					if len(document.Client.Endpoints) != 1 ||
						document.Client.Endpoints[0].Password != controlPassword {
						t.Fatalf(
							"client credential control characters were not preserved: %#v",
							document.Client.Endpoints,
						)
					}
				}
			}
			return fixture
		}
		registerMutation(registry, item.path, factory, factory)
	}
	modeFactory := func(t *testing.T) mutationFixture {
		return newClientModeMutationFixture(t)
	}
	registerMutation(registry, "xp2p client mode", modeFactory, modeFactory)
	registerClientSubscriptionMutationContracts(registry)
}

func newClientModeMutationFixture(t *testing.T) mutationFixture {
	t.Helper()
	restore := servicecontrol.SetDefaultForTesting(contractServiceController{mode: "inactive"})
	t.Cleanup(restore)
	content := strings.Replace(clientMutationBase(false), "[client]\n", `[client]
install_dir = "C:/xp2p-client"
tun_enabled = true
`, 1)
	fixture := newClientMutationFixture(
		t,
		content,
		[]string{"client", "mode", "proxy"},
		[]string{"client", "mode", "invalid"},
		nil,
	)
	desiredPath := filepath.Join(config.ConfigRoot(), layout.ClientConfigFileName)
	fixture.snapshot = func(t *testing.T) any {
		return snapshotModeMutation(t, desiredPath)
	}
	fixture.assertSuccess = func(t *testing.T, before, after any) {
		t.Helper()
		beforeState := before.(modeMutationSnapshot)
		afterState := after.(modeMutationSnapshot)
		if beforeState.desired == afterState.desired || !afterState.applyExists {
			t.Fatalf("client mode state was not fully staged: before=%#v after=%#v", beforeState, afterState)
		}
	}
	return fixture
}

func clientMutationBase(endpointDisabled bool) string {
	disabled := ""
	if endpointDisabled {
		disabled = "\ndisabled = true"
	}
	return `[client]
[[client.endpoints]]
profile = "trojan-tls"
protocol = "trojan"
transport = "tcp"
security = "tls"
hostname = "127.0.0.1"
tag = "edge-a"
address = "203.0.113.10"
port = 443
user = "matrix-user"
password = "initial-value"
server_name = "edge.example"` + disabled + `

[client.reverse.reverse-a]
tag = "reverse-a"
host = "reverse.example"
user_id = "reverse-user"
endpoint_tag = "edge-a"
`
}

func disabledClientReverseFixture() string {
	return strings.Replace(
		clientMutationBase(false),
		"endpoint_tag = \"edge-a\"\n",
		"endpoint_tag = \"edge-a\"\ndisabled = true\n",
		1,
	)
}

func newClientMutationFixture(
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
	path := filepath.Join(root, layout.ClientConfigFileName)
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
			if before == after {
				t.Fatal("client Desired document was not updated")
			}
		},
	}
}
