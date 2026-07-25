package root

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

var stage4ContractPaths = map[string]struct{}{
	"xp2p client debug bundle":       {},
	"xp2p client deploy":             {},
	"xp2p client export":             {},
	"xp2p client import":             {},
	"xp2p client install":            {},
	"xp2p server debug bundle":       {},
	"xp2p server export":             {},
	"xp2p server identity provision": {},
	"xp2p server import":             {},
	"xp2p server install":            {},
	"xp2p server user add":           {},
	"xp2p server user rotate":        {},
}

func TestStage4LeavesCovered(t *testing.T) {
	baseline := buildLegacyPendingBaseline()
	for path, legacy := range baseline {
		if legacy.coverage != contractStage4 {
			continue
		}
		if _, ok := stage4ContractPaths[path]; !ok {
			t.Errorf("stage 4 leaf has no executable contract: %s", path)
		}
		scenario := contractCaseRegistry[path]
		if scenario.coverage != contractCovered || !scenario.artifact {
			t.Errorf("stage 4 leaf is not covered: %s", path)
		}
	}
	for path, scenario := range contractCaseRegistry {
		if scenario.coverage == contractStage4 {
			t.Errorf("stage 4 pending status remains: %s", path)
		}
	}
}

func TestStage4ArchiveContractCases(t *testing.T) {
	for _, role := range []string{"client", "server"} {
		role := role
		t.Run(role, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			name := layout.ClientConfigFileName
			if role == "server" {
				name = layout.ServerConfigFileName
			}
			configPath := filepath.Join(root, name)
			if err := os.WriteFile(configPath, []byte("version = \"0.2.9\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			exportPath := filepath.Join(root, role+"-\u96ea.zip")
			assertStage4ArchiveSuccess(t, "xp2p "+role+" export",
				[]string{role, "export", "--config-root", root, "--output", exportPath}, exportPath)

			importRoot := filepath.Join(root, "imported")
			execution := executeContractCase([]string{
				role, "import", "--config-root", importRoot, "--input", exportPath,
			}, false)
			result := assertStage4Success(t, "xp2p "+role+" import", execution)
			if result["status"] != "completed" || result["path"] != exportPath {
				t.Fatalf("unexpected import result: %#v", result)
			}
			if _, err := os.Stat(filepath.Join(importRoot, name)); err != nil {
				t.Fatalf("imported artifact is absent: %v", err)
			}

			debugPath := filepath.Join(root, role+"-debug.zip")
			assertStage4ArchiveSuccess(t, "xp2p "+role+" debug bundle",
				[]string{role, "debug", "bundle", "--output", debugPath}, debugPath)

			failure := executeContractCase([]string{
				role, "import", "--config-root", filepath.Join(root, "failed"),
				"--input", filepath.Join(root, "missing.zip"),
			}, false)
			assertStage4Failure(t, "xp2p "+role+" import", failure, "matrix-secret")
		})
	}
}

func TestStage4ArchiveHumanCompatibility(t *testing.T) {
	for _, role := range []string{"client", "server"} {
		role := role
		t.Run(role, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			name := layout.ClientConfigFileName
			if role == "server" {
				name = layout.ServerConfigFileName
			}
			if err := os.WriteFile(filepath.Join(root, name), []byte("version = \"0.2.9\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			exportPath := filepath.Join(root, role+"-human.zip")
			stdout, stderr, err := executeHumanContractCase([]string{
				role, "export", "--config-root", root, "--output", exportPath,
			})
			assertStage4Human(t, stdout, stderr, err, "archive created", exportPath)

			importRoot := filepath.Join(root, "human-import")
			stdout, stderr, err = executeHumanContractCase([]string{
				role, "import", "--config-root", importRoot, "--input", exportPath,
			})
			assertStage4Human(t, stdout, stderr, err, "archive applied", "verify service status")

			debugPath := filepath.Join(root, role+"-human-debug.zip")
			stdout, stderr, err = executeHumanContractCase([]string{
				role, "debug", "bundle", "--output", debugPath,
			})
			assertStage4Human(t, stdout, stderr, err, "archive created", debugPath)
		})
	}
}

func TestStage4UserCredentialContractCases(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		newServerMutationFixture(t, serverMutationBase(false, false),
			nil, nil, nil)
		secret := "123e4567-e89b-12d3-a456-426614174099"
		execution := executeContractCase([]string{
			"server", "user", "add", "--id", "unicode-\u96ea", "--password", secret,
			"--host", "edge.example", "--no-reverse",
		}, false)
		result := assertStage4Success(t, "xp2p server user add", execution)
		if result["user_id"] != "unicode-\u96ea" || result["password"] != secret {
			t.Fatalf("required credential fields are missing: %#v", result)
		}
		link, _ := result["link"].(string)
		if !strings.Contains(link, secret) {
			t.Fatalf("credential link does not retain the explicit secret: %#v", result)
		}

		failureSecret := "invalid+credential"
		failure := executeContractCase([]string{
			"server", "user", "add", "--id", "bad", "--password", failureSecret,
		}, false)
		assertStage4Failure(t, "xp2p server user add", failure, failureSecret, secret)
	})

	t.Run("rotate", func(t *testing.T) {
		newServerMutationFixture(t, serverMutationBase(false, false),
			nil, nil, nil)
		execution := executeContractCase([]string{
			"server", "user", "rotate", "matrix-user", "--ttl", "1h",
		}, false)
		result := assertStage4Success(t, "xp2p server user rotate", execution)
		credential, _ := result["credential"].(string)
		if strings.TrimSpace(credential) == "" || credential == "initial-server-value" {
			t.Fatalf("rotation did not return a new credential: %#v", result)
		}
		expiry, _ := result["previous_valid_until"].(string)
		parsed, err := time.Parse(time.RFC3339, expiry)
		if err != nil || parsed.Location() != time.UTC {
			t.Fatalf("rotation expiry is not UTC RFC3339: %q err=%v", expiry, err)
		}

		failure := executeContractCase([]string{
			"server", "user", "rotate", "missing-user",
		}, false)
		assertStage4Failure(t, "xp2p server user rotate", failure,
			"initial-server-value", credential)
	})
}

func TestStage4CredentialHumanCompatibility(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		newServerMutationFixture(t, serverMutationBase(false, false), nil, nil, nil)
		secret := "123e4567-e89b-12d3-a456-426614174088"
		stdout, stderr, err := executeHumanContractCase([]string{
			"server", "user", "add", "--id", "human-user", "--password", secret,
			"--host", "edge.example", "--no-reverse",
		})
		assertStage4Human(t, stdout, stderr, err, secret, "trojan://")
	})
	t.Run("rotate", func(t *testing.T) {
		newServerMutationFixture(t, serverMutationBase(false, false), nil, nil, nil)
		stdout, stderr, err := executeHumanContractCase([]string{
			"server", "user", "rotate", "matrix-user", "--ttl", "1h",
		})
		assertStage4Human(t, stdout, stderr, err, "Credential:", "Previous valid until:")
	})
}

func TestStage4IdentityProvisionContractCase(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	certPath := filepath.Join(root, "server.pem")
	keyPath := filepath.Join(root, "server.key")
	writeContractCertificate(t, certPath, keyPath, "edge.example", []string{"edge.example"}, nil)
	desired := "[server]\n" +
		"host = \"edge.example\"\n" +
		"trojan_port = \"58443\"\n" +
		"certificate = " + strconvQuote(certPath) + "\n" +
		"key = " + strconvQuote(keyPath) + "\n"
	if err := os.WriteFile(filepath.Join(root, layout.ServerConfigFileName), []byte(desired), 0o600); err != nil {
		t.Fatal(err)
	}
	label := "idp-unicode-\u96ea@xp2p.local"
	store := identitysync.DefaultStore()
	if err := store.Save(identitysync.State{
		SchemaVersion: identitysync.SchemaVersion,
		Provider:      &identitysync.ProviderRef{InstanceID: "matrix", Kind: identitysync.ProviderSCIM},
		Current: &identitysync.Generation{
			ID: "generation-1", ProviderInstanceID: "matrix",
			Subjects: map[string]identitysync.Subject{
				"subject-1": {ExternalSubject: "subject-1", UserLabel: label, Active: true},
			},
			Groups: map[string]identitysync.Group{},
		},
		Status: identitysync.Status{State: identitysync.SyncStatusSuccess},
	}); err != nil {
		t.Fatal(err)
	}

	execution := executeContractCase([]string{
		"server", "identity", "provision", label, "--host", "edge.example",
	}, false)
	result := assertStage4Success(t, "xp2p server identity provision", execution)
	if result["user_id"] != label {
		t.Fatalf("unexpected provisioned identity: %#v", result)
	}
	link, _ := result["link"].(string)
	if !strings.HasPrefix(link, "trojan://") || !strings.Contains(link, "edge.example") {
		t.Fatalf("provision result does not contain the generated credential link: %#v", result)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Current.Subjects["subject-1"].Provisioned {
		t.Fatal("identity cache was not marked provisioned")
	}

	failure := executeContractCase([]string{
		"server", "identity", "provision", "idp-missing@xp2p.local",
	}, false)
	assertStage4Failure(t, "xp2p server identity provision", failure, link)
	if _, err := os.Stat(config.ApplyErrorPath()); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func strconvQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
