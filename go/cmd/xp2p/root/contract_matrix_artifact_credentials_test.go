package root

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func userAddStage4Contract() stage4Contract {
	return stage4Contract{
		success: func(t *testing.T, path string) {
			newServerMutationFixture(t, serverMutationBase(false, false), nil, nil, nil)
			secret := "123e4567-e89b-12d3-a456-426614174099"
			execution := executeContractCase([]string{
				"server", "user", "add", "--id", "unicode-\u96ea", "--password", secret,
				"--host", "edge.example", "--no-reverse",
			}, false)
			result := assertStage4Success(t, path, execution)
			if result["user_id"] != "unicode-\u96ea" || result["password"] != secret {
				t.Fatalf("required credential fields are missing: %#v", result)
			}
			link, _ := result["link"].(string)
			if !strings.Contains(link, secret) {
				t.Fatalf("credential link does not retain the explicit secret: %#v", result)
			}
		},
		failure: func(t *testing.T, path string) {
			newServerMutationFixture(t, serverMutationBase(false, false), nil, nil, nil)
			secret := "invalid+credential"
			execution := executeContractCase([]string{
				"server", "user", "add", "--id", "bad", "--password", secret,
			}, false)
			assertStage4Failure(t, path, execution, secret, "initial-server-value")
		},
		human: func(t *testing.T, path string) {
			newServerMutationFixture(t, serverMutationBase(false, false), nil, nil, nil)
			secret := "123e4567-e89b-12d3-a456-426614174088"
			stdout, stderr, err := executeHumanContractCase([]string{
				"server", "user", "add", "--id", "human-user", "--password", secret,
				"--host", "edge.example", "--no-reverse",
			})
			assertStage4Human(t, path, stdout, stderr, err, secret, "trojan://")
		},
	}
}

func userRotateStage4Contract() stage4Contract {
	return stage4Contract{
		success: func(t *testing.T, path string) {
			newServerMutationFixture(t, serverMutationBase(false, false), nil, nil, nil)
			execution := executeContractCase([]string{
				"server", "user", "rotate", "matrix-user", "--ttl", "1h",
			}, false)
			result := assertStage4Success(t, path, execution)
			credential, _ := result["credential"].(string)
			if strings.TrimSpace(credential) == "" || credential == "initial-server-value" {
				t.Fatalf("rotation did not return a new credential: %#v", result)
			}
			expiry, _ := result["previous_valid_until"].(string)
			parsed, err := time.Parse(time.RFC3339, expiry)
			if err != nil || parsed.Location() != time.UTC {
				t.Fatalf("rotation expiry is not UTC RFC3339: %q err=%v", expiry, err)
			}
		},
		failure: func(t *testing.T, path string) {
			newServerMutationFixture(t, serverMutationBase(false, false), nil, nil, nil)
			execution := executeContractCase([]string{
				"server", "user", "rotate", "missing-user",
			}, false)
			assertStage4Failure(t, path, execution, "initial-server-value")
		},
		human: func(t *testing.T, path string) {
			newServerMutationFixture(t, serverMutationBase(false, false), nil, nil, nil)
			stdout, stderr, err := executeHumanContractCase([]string{
				"server", "user", "rotate", "matrix-user", "--ttl", "1h",
			})
			assertStage4Human(t, path, stdout, stderr, err, "Credential:", "Previous valid until:")
		},
	}
}

func identityProvisionStage4Contract() stage4Contract {
	return stage4Contract{
		success: func(t *testing.T, path string) {
			fixture := newStage4IdentityFixture(t)
			execution := executeContractCase([]string{
				"server", "identity", "provision", fixture.label, "--host", "edge.example",
			}, false)
			result := assertStage4Success(t, path, execution)
			link, _ := result["link"].(string)
			if result["user_id"] != fixture.label || !strings.HasPrefix(link, "trojan://") {
				t.Fatalf("unexpected identity provision result: %#v", result)
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if !state.Current.Subjects["subject-1"].Provisioned {
				t.Fatal("identity cache was not marked provisioned")
			}
		},
		failure: func(t *testing.T, path string) {
			newStage4IdentityFixture(t)
			execution := executeContractCase([]string{
				"server", "identity", "provision", "idp-missing@xp2p.local",
			}, false)
			assertStage4Failure(t, path, execution, "stage4-identity-secret")
		},
		human: func(t *testing.T, path string) {
			fixture := newStage4IdentityFixture(t)
			stdout, stderr, err := executeHumanContractCase([]string{
				"server", "identity", "provision", fixture.label, "--host", "edge.example",
			})
			assertStage4Human(t, path, stdout, stderr, err, "trojan://")
		},
	}
}

type stage4IdentityFixture struct {
	label string
	store identitysync.Store
}

func newStage4IdentityFixture(t *testing.T) stage4IdentityFixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	certPath := filepath.Join(root, "server.pem")
	keyPath := filepath.Join(root, "server.key")
	writeContractCertificate(t, certPath, keyPath, "edge.example", []string{"edge.example"}, nil)
	desired := "[server]\nhost = \"edge.example\"\ntrojan_port = \"58443\"\n" +
		"certificate = " + stage4Quote(certPath) + "\nkey = " + stage4Quote(keyPath) + "\n"
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
	return stage4IdentityFixture{label: label, store: store}
}

func stage4Quote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
