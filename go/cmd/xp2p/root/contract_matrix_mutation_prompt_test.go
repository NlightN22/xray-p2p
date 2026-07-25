package root

import (
	"reflect"
	"strings"
	"testing"
)

func TestStage3RedirectMutationsDoNotPromptInJSONMode(t *testing.T) {
	type promptCase struct {
		path    string
		args    []string
		fixture func(*testing.T) mutationFixture
	}
	clientAdd := clientMutationBase(false) + secondClientEndpoint()
	clientMatches := clientAdd + ambiguousClientRedirects(false)
	clientDisabledMatches := clientAdd + ambiguousClientRedirects(true)
	serverAdd := serverMutationBase(false, false) + secondServerReverse()
	serverMatches := serverAdd + ambiguousServerRedirects(false)
	serverDisabledMatches := serverAdd + ambiguousServerRedirects(true)
	cases := []promptCase{
		{"xp2p client redirect add", []string{"client", "redirect", "add", "--domain", "prompt.example"},
			clientPromptFixture(clientAdd)},
		{"xp2p client redirect remove", []string{"client", "redirect", "remove", "--domain", "prompt.example"},
			clientPromptFixture(clientMatches)},
		{"xp2p client redirect disable", []string{"client", "redirect", "disable", "--domain", "prompt.example"},
			clientPromptFixture(clientMatches)},
		{"xp2p client redirect enable", []string{"client", "redirect", "enable", "--domain", "prompt.example"},
			clientPromptFixture(clientDisabledMatches)},
		{"xp2p server redirect add", []string{"server", "redirect", "add", "--domain", "prompt.example"},
			serverPromptFixture(serverAdd)},
		{"xp2p server redirect remove", []string{"server", "redirect", "remove", "--domain", "prompt.example"},
			serverPromptFixture(serverMatches)},
		{"xp2p server redirect disable", []string{"server", "redirect", "disable", "--domain", "prompt.example"},
			serverPromptFixture(serverMatches)},
		{"xp2p server redirect enable", []string{"server", "redirect", "enable", "--domain", "prompt.example"},
			serverPromptFixture(serverDisabledMatches)},
	}
	for _, item := range cases {
		item := item
		t.Run(item.path, func(t *testing.T) {
			fixture := item.fixture(t)
			before := fixture.snapshot(t)
			execution := executeContractCase(item.args, false)
			assertMutationFailure(t, item.path, execution, nil)
			after := fixture.snapshot(t)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("non-interactive prompt rejection changed Desired")
			}
			raw := strings.ToLower(execution.stdout + execution.stderr)
			for _, prompt := range []string{"select ", "choose ", "available client", "available reverse"} {
				if strings.Contains(raw, prompt) {
					t.Fatalf("JSON mode emitted prompt text %q: %q", prompt, raw)
				}
			}
		})
	}
}

func TestStage3ClientModeFullDoesNotPromptInJSONMode(t *testing.T) {
	fixture := newClientMutationFixture(
		t,
		strings.Replace(
			clientMutationBase(false)+secondClientEndpoint(),
			"[client]\n",
			"[client]\ninstall_dir = \"C:/xp2p-client\"\n",
			1,
		),
		nil,
		nil,
		nil,
	)
	before := fixture.snapshot(t)
	execution := executeContractCase([]string{"client", "mode", "tun", "full"}, false)
	assertMutationFailure(t, "xp2p client mode", execution, nil)
	after := fixture.snapshot(t)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("non-interactive full-tunnel selection changed Desired")
	}
	raw := strings.ToLower(execution.stdout + execution.stderr)
	for _, prompt := range []string{"select ", "choose ", "available client"} {
		if strings.Contains(raw, prompt) {
			t.Fatalf("JSON mode emitted prompt text %q: %q", prompt, raw)
		}
	}
}

func clientPromptFixture(content string) func(*testing.T) mutationFixture {
	return func(t *testing.T) mutationFixture {
		return newClientMutationFixture(t, content, nil, nil, nil)
	}
}

func serverPromptFixture(content string) func(*testing.T) mutationFixture {
	return func(t *testing.T) mutationFixture {
		return newServerMutationFixture(t, content, nil, nil, nil)
	}
}

func secondClientEndpoint() string {
	return `
[[client.endpoints]]
profile = "trojan-tls"
protocol = "trojan"
transport = "tcp"
security = "tls"
hostname = "127.0.0.2"
tag = "edge-b"
address = "203.0.113.11"
port = 443
user = "matrix-user-b"
password = "prompt-value"
server_name = "edge-b.example"
`
}

func ambiguousClientRedirects(disabled bool) string {
	state := ""
	if disabled {
		state = "\ndisabled = true"
	}
	return `
[[client.redirects]]
domain = "prompt.example"
outbound_tag = "edge-a"` + state + `

[[client.redirects]]
domain = "prompt.example"
outbound_tag = "edge-b"` + state + "\n"
}

func secondServerReverse() string {
	return `
[server.reverse_channels.reverse-b]
domain = "reverse-b.example"
host = "127.0.0.2"
user_id = "matrix-user"
tag = "reverse-b"
`
}

func ambiguousServerRedirects(disabled bool) string {
	state := ""
	if disabled {
		state = "\ndisabled = true"
	}
	return `
[[server.server_redirects]]
domain = "prompt.example"
outbound_tag = "reverse-a"` + state + `

[[server.server_redirects]]
domain = "prompt.example"
outbound_tag = "reverse-b"` + state + "\n"
}
