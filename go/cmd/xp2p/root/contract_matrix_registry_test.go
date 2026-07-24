package root

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

const legacyPendingBaselineDigest = "c1f4c0b41b5c21597797deef0d556d01540703102836557b6a8385d9f3dad158"

type contractCoverage string

const (
	contractCovered contractCoverage = "covered"
	contractStage2  contractCoverage = "pending:2"
	contractStage3  contractCoverage = "pending:3"
	contractStage4  contractCoverage = "pending:4"
	contractStage5  contractCoverage = "pending:5"
	contractStage6  contractCoverage = "pending:6"
)

type contractCase struct {
	coverage         contractCoverage
	success          []string
	empty            []string
	failure          []string
	setup            func(*testing.T, string)
	assertResult     func(*testing.T, map[string]any)
	assertEmpty      func(*testing.T, map[string]any)
	emptyResult      string
	credentialPolicy string
	edgeCases        []string
	platform         string
	human            []string
	assertHuman      func(*testing.T, string, string)
}

var contractCaseRegistry = buildContractCaseRegistry()

func buildContractCaseRegistry() map[string]contractCase {
	baseline := buildLegacyPendingBaseline()
	registry := make(map[string]contractCase, len(baseline)+1)
	for path, scenario := range baseline {
		registry[path] = scenario
	}
	registry["xp2p client list"] = clientListContractCase()
	registry["xp2p client forward list"] = forwardListContractCase("client")
	registry["xp2p client group list"] = clientGroupListContractCase()
	registry["xp2p client obs"] = clientObsContractCase()
	registry["xp2p client redirect list"] = redirectListContractCase("client")
	registry["xp2p client reverse list"] = clientReverseListContractCase()
	registry["xp2p client subscription offers"] = clientSubscriptionContractCase("offers")
	registry["xp2p client subscription status"] = clientSubscriptionContractCase("status")
	registry["xp2p client state"] = clientStateContractCase()
	registry["xp2p server forward list"] = forwardListContractCase("server")
	registry["xp2p server ha peer list"] = serverHAPeerListContractCase()
	registry["xp2p server ha member list"] = serverHACollectionContractCase("member")
	registry["xp2p server ha channel list"] = serverHACollectionContractCase("channel")
	registry["xp2p server ha group inspect"] = serverHAGroupInspectContractCase()
	registry["xp2p server ha redirect list"] = serverHARedirectListContractCase()
	registry["xp2p server ha status"] = serverHAStatusContractCase()
	registry["xp2p server redirect list"] = redirectListContractCase("server")
	registry["xp2p server reverse list"] = serverReverseListContractCase()
	registry["xp2p server state"] = serverStateContractCase()
	registry["xp2p server user list"] = serverUserListContractCase()
	return registry
}

func pendingBaselineDigest(baseline map[string]contractCase) string {
	paths := make([]string, 0, len(baseline))
	for path := range baseline {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		_, _ = fmt.Fprintf(hash, "%s=%s\n", path, baseline[path].coverage)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func buildLegacyPendingBaseline() map[string]contractCase {
	registry := make(map[string]contractCase)
	registerPending := func(coverage contractCoverage, paths ...string) {
		for _, path := range paths {
			if _, exists := registry[path]; exists {
				panic("duplicate contract case: " + path)
			}
			registry[path] = contractCase{coverage: coverage}
		}
	}

	registerPending(contractStage2, strings.Fields(`
xp2p.client.forward.list xp2p.client.group.list
xp2p.client.obs xp2p.client.redirect.list
xp2p.client.reverse.list xp2p.client.state xp2p.client.subscription.offers
xp2p.client.subscription.status xp2p.server.cert.state
xp2p.server.forward.list
xp2p.server.ha.channel.inspect xp2p.server.ha.channel.list
xp2p.server.ha.group.inspect xp2p.server.ha.member.list
xp2p.server.ha.peer.list xp2p.server.ha.redirect.list xp2p.server.ha.status
xp2p.server.identity.status xp2p.server.profile xp2p.server.redirect.list
xp2p.server.reverse.list xp2p.server.state xp2p.server.user.list
xp2p.heartbeat.contract`)...)

	registerPending(contractStage3, strings.Fields(`
xp2p.client.disable xp2p.client.enable xp2p.client.forward.add
xp2p.client.forward.remove xp2p.client.redirect.add xp2p.client.redirect.disable
xp2p.client.redirect.enable xp2p.client.redirect.remove
xp2p.client.reverse.disable xp2p.client.reverse.enable
xp2p.client.subscription.add xp2p.client.subscription.refresh
xp2p.client.subscription.remove xp2p.client.update
xp2p.server.forward.add xp2p.server.forward.remove
xp2p.server.ha.channel.create xp2p.server.ha.channel.disable
xp2p.server.ha.channel.finalize xp2p.server.ha.channel.rebind
xp2p.server.ha.channel.rebind-endpoint xp2p.server.ha.group.create
xp2p.server.ha.group.remove xp2p.server.ha.group.update
xp2p.server.ha.member.add xp2p.server.ha.member.remove
xp2p.server.ha.member.reprioritize xp2p.server.ha.peer.add
xp2p.server.ha.peer.remove xp2p.server.ha.peer.self
xp2p.server.ha.redirect.add xp2p.server.ha.redirect.remove
xp2p.server.ha.sync xp2p.server.identity.detach xp2p.server.identity.select
xp2p.server.identity.sync xp2p.server.redirect.access.add-group
xp2p.server.redirect.access.add-user xp2p.server.redirect.access.clear
xp2p.server.redirect.access.remove-group xp2p.server.redirect.access.remove-user
xp2p.server.redirect.access.set xp2p.server.redirect.add
xp2p.server.redirect.disable xp2p.server.redirect.enable
xp2p.server.redirect.remove xp2p.server.reverse.disable
xp2p.server.reverse.enable xp2p.server.user.disable xp2p.server.user.enable
xp2p.server.user.remove xp2p.server.user.update`)...)

	registerPending(contractStage4, strings.Fields(`
xp2p.client.debug.bundle xp2p.client.deploy xp2p.client.export
xp2p.client.import xp2p.client.install xp2p.server.debug.bundle
xp2p.server.export xp2p.server.identity.provision xp2p.server.import
xp2p.server.install xp2p.server.user.add xp2p.server.user.rotate`)...)

	registerPending(contractStage5, strings.Fields(`
xp2p.client.mode xp2p.client.remove xp2p.client.service.restart
xp2p.client.service.start xp2p.client.service.status xp2p.client.service.stop
xp2p.server.cert.set xp2p.server.mode xp2p.server.remove
xp2p.server.service.restart xp2p.server.service.start
xp2p.server.service.status xp2p.server.service.stop`)...)

	registerPending(contractStage6, strings.Fields(`
xp2p.client.dns-forward.add xp2p.client.dns-forward.list
xp2p.client.dns-forward.remove xp2p.server.dns-forward.add
xp2p.server.dns-forward.list xp2p.server.dns-forward.remove
xp2p.nat-redirect.add xp2p.nat-redirect.list xp2p.nat-redirect.remove`)...)

	for path, item := range registry {
		delete(registry, path)
		registry[strings.ReplaceAll(path, ".", " ")] = item
	}
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
