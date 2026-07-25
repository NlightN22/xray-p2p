package root

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

type contractCoverage string

const (
	contractCovered contractCoverage = "covered"
)

type contractCase struct {
	coverage         contractCoverage
	mutation         bool
	artifact         bool
	platformCase     bool
	success          []string
	empty            []string
	failure          []string
	cancelFailure    bool
	setup            func(*testing.T, string)
	assertResult     func(*testing.T, map[string]any)
	assertEmpty      func(*testing.T, map[string]any)
	emptyResult      string
	credentialPolicy string
	edgeCases        []string // Documentation only; executable checks live in assertEdgeCases.
	assertEdgeCases  func(*testing.T, map[string]any, string, string)
	platform         string
	human            []string
	assertHuman      func(*testing.T, string, string)
}

var contractCaseRegistry = buildContractCaseRegistry()

func buildContractCaseRegistry() map[string]contractCase {
	registry := make(map[string]contractCase, len(outputContractInventory))
	registry["xp2p client list"] = clientListContractCase()
	registry["xp2p client dns-forward list"] = dnsForwardListContractCase("client")
	registry["xp2p client mode"] = modeReadContractCase("client")
	registry["xp2p client forward list"] = forwardListContractCase("client")
	registry["xp2p client group list"] = clientGroupListContractCase()
	registry["xp2p client obs"] = clientObsContractCase()
	registry["xp2p client redirect list"] = redirectListContractCase("client")
	registry["xp2p client reverse list"] = clientReverseListContractCase()
	registry["xp2p client service status"] = serviceStatusContractCase("client")
	registry["xp2p client subscription offers"] = clientSubscriptionContractCase("offers")
	registry["xp2p client subscription status"] = clientSubscriptionContractCase("status")
	registry["xp2p client state"] = clientStateContractCase()
	registry["xp2p server forward list"] = forwardListContractCase("server")
	registry["xp2p server cert state"] = serverCertStateContractCase()
	registry["xp2p server ha peer list"] = serverHAPeerListContractCase()
	registry["xp2p server ha member list"] = serverHACollectionContractCase("member")
	registry["xp2p server ha channel inspect"] = serverHAChannelInspectContractCase()
	registry["xp2p server ha channel list"] = serverHACollectionContractCase("channel")
	registry["xp2p server ha group inspect"] = serverHAGroupInspectContractCase()
	registry["xp2p server ha redirect list"] = serverHARedirectListContractCase()
	registry["xp2p server ha status"] = serverHAStatusContractCase()
	registry["xp2p server identity status"] = serverIdentityStatusContractCase()
	registry["xp2p server mode"] = modeReadContractCase("server")
	registry["xp2p server profile"] = serverProfileContractCase()
	registry["xp2p server redirect list"] = redirectListContractCase("server")
	registry["xp2p server reverse list"] = serverReverseListContractCase()
	registry["xp2p server service status"] = serviceStatusContractCase("server")
	registry["xp2p server state"] = serverStateContractCase()
	registry["xp2p server user list"] = serverUserListContractCase()
	registry["xp2p heartbeat contract"] = heartbeatContractCase()
	registry["xp2p nat-redirect list"] = natRedirectListContractCase()
	registry["xp2p server dns-forward list"] = dnsForwardListContractCase("server")
	for path := range mutationContractRegistry {
		if path == "xp2p client mode" || path == "xp2p server mode" {
			continue
		}
		registry[path] = mutationCase()
	}
	for path := range stage4ContractRegistry {
		registerStage4ContractCase(registry, path)
	}
	for path, scenario := range registry {
		if scenario.coverage == contractCovered && scenario.assertEdgeCases == nil {
			scenario.assertEdgeCases = assertReadOnlyEdgeCases
			registry[path] = scenario
		}
	}
	registerStage5ContractCases(registry)
	registerStage6ContractCases(registry)
	return registry
}

func clientListContractCase() contractCase {
	return contractCase{
		coverage: contractCovered,
		success:  []string{"client", "list"},
		empty:    []string{"client", "list"},
		failure:  []string{"client", "list"},
		setup: func(t *testing.T, mode string) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			path := filepath.Join(root, layout.ClientConfigFileName)
			if mode == "error" {
				fixture := `[client]
[[client.endpoints]]
hostname = "edge.example"
tag = "primary"
heartbeat_mode = "unsupported"
`
				if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
					t.Fatal(err)
				}
				return
			}
			if mode == "empty" {
				return
			}
			fixture := `[client]
[[client.endpoints]]
profile = "trojan-tls"
protocol = "trojan"
transport = "tcp"
security = "tls"
hostname = "edge.example"
tag = "primary"
address = "203.0.113.10"
port = 443
user = "Miyuki\n\t\u0001"
password = "matrix-secret"
server_name = "edge.example"
allow_insecure = false
`
			if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		assertResult: func(t *testing.T, result map[string]any) {
			endpoints, ok := result["endpoints"].([]any)
			if !ok || len(endpoints) != 1 {
				t.Fatalf("endpoints=%#v", result["endpoints"])
			}
			endpoint, ok := endpoints[0].(map[string]any)
			if !ok || endpoint["port"] != float64(443) || endpoint["enabled"] != true ||
				endpoint["user"] != "Miyuki\n\t\u0001" {
				t.Fatalf("endpoint JSON types changed: %#v", endpoints[0])
			}
			if _, leaked := result["links"]; leaked {
				t.Fatalf("default list leaked credential links: %#v", result)
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			endpoints, ok := result["endpoints"].([]any)
			if !ok || endpoints == nil || len(endpoints) != 0 {
				t.Fatalf("empty endpoints must be []: %#v", result["endpoints"])
			}
		},
		emptyResult:      "endpoints is a non-nil empty array when no Desired file exists",
		credentialPolicy: "default list omits credential links and passwords",
		edgeCases:        []string{"number", "boolean", "Unicode/control characters", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            []string{"client", "list"},
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{
				"HOSTNAME", "TAG", "ADDRESS", "PORT", "USER", "TLS MODE", "SERVER NAME", "STATE",
				"edge.example", "primary", "203.0.113.10", "443", "Miyuki", "enabled",
			} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: %q", expected, output)
				}
			}
			if !strings.Contains(diagnostics, "INFO xp2p starting") {
				t.Fatalf("human diagnostic baseline changed: %q", diagnostics)
			}
		},
	}
}
