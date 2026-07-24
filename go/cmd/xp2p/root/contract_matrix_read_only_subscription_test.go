package root

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func clientSubscriptionContractCase(command string) contractCase {
	args := []string{"client", "subscription", command}
	resultKey := "subscriptions"
	if command == "offers" {
		resultKey = "offers"
	}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup:    setupClientSubscriptionCase,
		assertResult: func(t *testing.T, result map[string]any) {
			items, ok := result[resultKey].([]any)
			if !ok {
				t.Fatalf("%s=%#v", resultKey, result[resultKey])
			}
			if command == "status" {
				assertSubscriptionStatusResult(t, items)
			} else {
				assertSubscriptionOffersResult(t, items)
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"matrix-credential", "credential", "password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("subscription %s leaked credentials: %#v", command, result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			items, ok := result[resultKey].([]any)
			if !ok || items == nil || len(items) != 0 {
				t.Fatalf("empty %s must be []: %#v", resultKey, result[resultKey])
			}
		},
		emptyResult:      resultKey + " is a non-nil empty array when no subscriptions exist",
		credentialPolicy: "status and offers omit connection credentials and source URLs",
		edgeCases:        []string{"number", "UTC RFC3339", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			expected := []string{"source Ω", "offer-zulu", "offer-alpha"}
			if command == "status" {
				expected = []string{
					"ID: source Ω", "Adapter: 3x-ui", "Revision: rev-7", "Offers: 2",
					"Selected offer: offer-alpha", "Last refresh: 2026-07-24 10:20:30 +0000 UTC",
				}
			}
			for _, value := range expected {
				if !strings.Contains(output, value) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", value, output, diagnostics)
				}
			}
			if !strings.Contains(diagnostics, "INFO xp2p starting") {
				t.Fatalf("human diagnostic baseline changed: %q", diagnostics)
			}
		},
	}
}

func setupClientSubscriptionCase(t *testing.T, mode string) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	if mode == "empty" {
		return
	}
	desired := `[client]
[[client.subscriptions]]
id = "source Ω"
adapter = "3x-ui"
compatibility_version = "1"
url = "https://subscription.example/private"
selected_offer_id = "offer-alpha"
`
	writeContractFixture(t, filepath.Join(root, layout.ClientConfigFileName), desired)
	stateDir := filepath.Join(root, layout.StateDirName, "subscriptions")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	state := `not-json`
	if mode == "success" {
		state = `{
  "id": "source Ω",
  "adapter": "3x-ui",
  "revision": "rev-7",
  "selected_offer_id": "offer-alpha",
  "last_refresh_at": "2026-07-24T10:20:30Z",
  "last_apply_at": "2026-07-24T10:21:30Z",
  "last_error": "safe warning",
  "last_good": {
    "source": {"id": "source Ω", "adapter": "3x-ui"},
    "revision": "rev-7",
    "fetched_at": "2026-07-24T10:20:00Z",
    "offers": [
      {
        "stable_id": "offer-zulu",
        "endpoint": {"host": "zulu.example", "port": 443, "profile": "trojan-tls", "protocol": "trojan", "transport": "tcp", "security": "tls"},
        "user_label": "Zulu user",
        "credential": "matrix-credential-zulu"
      },
      {
        "stable_id": "offer-alpha",
        "endpoint": {"host": "alpha Ω.example", "port": 8443, "profile": "trojan-tls", "protocol": "trojan", "transport": "tcp", "security": "tls"},
        "user_label": "Alpha user",
        "credential": "matrix-credential-alpha"
      }
    ]
  }
}
`
	}
	writeContractFixture(t, filepath.Join(stateDir, "source Ω.json"), state)
}

func assertSubscriptionStatusResult(t *testing.T, items []any) {
	t.Helper()
	if len(items) != 1 {
		t.Fatalf("subscriptions=%#v", items)
	}
	status, ok := items[0].(map[string]any)
	if !ok || status["id"] != "source Ω" || status["adapter"] != "3x-ui" ||
		status["revision"] != "rev-7" || status["offer_count"] != float64(2) ||
		status["selected_offer_id"] != "offer-alpha" ||
		status["last_refresh_at"] != "2026-07-24T10:20:30Z" ||
		status["last_apply_at"] != "2026-07-24T10:21:30Z" {
		t.Fatalf("subscription status changed: %#v", items[0])
	}
}

func assertSubscriptionOffersResult(t *testing.T, items []any) {
	t.Helper()
	if len(items) != 2 {
		t.Fatalf("offers=%#v", items)
	}
	zulu, ok := items[0].(map[string]any)
	if !ok || zulu["subscription_id"] != "source Ω" || zulu["stable_id"] != "offer-zulu" ||
		zulu["port"] != float64(443) || zulu["user_label"] != "Zulu user" {
		t.Fatalf("first offer changed: %#v", items[0])
	}
	alpha, ok := items[1].(map[string]any)
	if !ok || alpha["stable_id"] != "offer-alpha" || alpha["host"] != "alpha Ω.example" ||
		alpha["port"] != float64(8443) {
		t.Fatalf("offer order or types changed: %#v", items[1])
	}
}
