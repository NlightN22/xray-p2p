package root

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func serverIdentityStatusContractCase() contractCase {
	args := []string{"server", "identity", "status"}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup:    setupServerIdentityStatusCase,
		assertResult: func(t *testing.T, result map[string]any) {
			if result["status"] != "success" || result["last_success"] != "2025-01-02T03:04:05Z" ||
				result["provider_id"] != "corp Ω" || result["provider_kind"] != "ldap" ||
				result["generation"] != "gen-7" || result["detached"] != false {
				t.Fatalf("identity status header changed: %#v", result)
			}
			subjects, ok := result["subjects"].([]any)
			if !ok || len(subjects) != 2 {
				t.Fatalf("subjects=%#v", result["subjects"])
			}
			alpha, ok := subjects[0].(map[string]any)
			if !ok || alpha["label"] != "alpha user" || alpha["active"] != true ||
				alpha["provisioned"] != true {
				t.Fatalf("identity subject order or booleans changed: %#v", subjects[0])
			}
			groups, ok := result["groups"].([]any)
			if !ok || len(groups) != 2 {
				t.Fatalf("groups=%#v", result["groups"])
			}
			engineers, ok := groups[0].(map[string]any)
			if !ok || engineers["id"] != "engineers" {
				t.Fatalf("identity group order changed: %#v", groups[0])
			}
			transitive, ok := engineers["transitive_members"].([]any)
			if !ok || len(transitive) != 2 || transitive[0] != "alpha" || transitive[1] != "zulu" {
				t.Fatalf("transitive members changed: %#v", engineers["transitive_members"])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"matrix-secret", "password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("identity status leaked credentials: %#v", result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			if result["status"] != "never" {
				t.Fatalf("empty identity status changed: %#v", result)
			}
			for _, key := range []string{"subjects", "groups", "redirects"} {
				items, ok := result[key].([]any)
				if !ok || items == nil || len(items) != 0 {
					t.Fatalf("empty %s must be []: %#v", key, result[key])
				}
			}
		},
		emptyResult:      "subjects, groups, and redirects are non-nil empty arrays before the first sync",
		credentialPolicy: "status omits provider credentials and provisioned user secrets",
		edgeCases:        []string{"boolean", "ordered and nested collections", "UTC timestamp", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"status: success", "provider: corp Ω (ldap)", "generation: gen-7", "user alpha user", "group engineers"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
				}
			}
		},
	}
}

func setupServerIdentityStatusCase(t *testing.T, mode string) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	configFixture := fmt.Sprintf("[server]\ninstall_dir = %q\n", filepath.ToSlash(root))
	writeContractFixture(t, filepath.Join(root, layout.ServerConfigFileName), configFixture)
	if mode == "empty" {
		return
	}
	statePath := config.IdentityStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("create identity state directory: %v", err)
	}
	stateFixture := "{invalid"
	if mode == "success" {
		stateFixture = `{
  "schema_version": 1,
  "provider": {"instance_id": "corp Ω", "kind": "ldap"},
  "current": {
    "id": "gen-7",
    "provider_instance_id": "corp Ω",
    "created_at": "2025-01-02T03:04:05Z",
    "subjects": {
      "zulu": {"external_subject": "external-zulu", "user_label": "zulu Ω user", "active": false, "direct_groups": ["platform"]},
      "alpha": {"external_subject": "external-alpha", "user_label": "alpha user", "active": true, "provisioned": true, "direct_groups": ["engineers"]}
    },
    "groups": {
      "platform": {"id": "platform", "direct_members": ["zulu"]},
      "engineers": {"id": "engineers", "direct_members": ["alpha"], "direct_groups": ["platform"]}
    }
  },
  "status": {"state": "success", "last_success": "2025-01-02T03:04:05Z"}
}`
	}
	writeContractFixture(t, statePath, stateFixture)
}
