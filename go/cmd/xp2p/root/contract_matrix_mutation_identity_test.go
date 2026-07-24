package root

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func registerIdentityMutationContracts(registry map[string]mutationContract) {
	registerMutation(
		registry,
		"xp2p server identity select",
		func(t *testing.T) mutationFixture {
			return newIdentityStateMutationFixture(t,
				[]string{"server", "identity", "select", "provider-例", "--kind", "scim", "--group", "engineering"},
				[]string{"server", "identity", "select", "provider-bad", "--kind", "unknown"},
				false,
			)
		},
		func(t *testing.T) mutationFixture {
			return newIdentityStateMutationFixture(t,
				[]string{"server", "identity", "select", "provider-例", "--kind", "scim"},
				[]string{"server", "identity", "select", "provider-bad", "--kind", "unknown"},
				false,
			)
		},
	)
	registerMutation(
		registry,
		"xp2p server identity detach",
		func(t *testing.T) mutationFixture {
			fixture := newIdentityStateMutationFixture(t,
				[]string{"server", "identity", "detach"},
				[]string{"server", "identity", "detach"},
				false,
			)
			mustRunMutationPrerequisite(t,
				[]string{"server", "identity", "select", "provider-a", "--kind", "scim"})
			return fixture
		},
		func(t *testing.T) mutationFixture {
			return newIdentityStateMutationFixture(t,
				[]string{"server", "identity", "detach"},
				[]string{"server", "identity", "detach"},
				true,
			)
		},
	)
	registerMutation(
		registry,
		"xp2p server identity sync",
		func(t *testing.T) mutationFixture {
			server := newSCIMFixtureServer(t, false)
			return newIdentitySyncMutationFixture(t, server.URL, true)
		},
		func(t *testing.T) mutationFixture {
			server := newSCIMFixtureServer(t, true)
			return newIdentitySyncMutationFixture(t, server.URL, false)
		},
	)
}

func newSCIMFixtureServer(t *testing.T, fail bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/users"):
			_, _ = w.Write([]byte(`{"totalResults":1,"startIndex":1,"itemsPerPage":1,"Resources":[{"id":"subject-1","userName":"matrix","firstName":"例","lastName":"User"}]}`))
		case strings.Contains(r.URL.Path, "/groups"):
			_, _ = w.Write([]byte(`{"totalResults":0,"startIndex":1,"itemsPerPage":0,"Resources":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newIdentityStateMutationFixture(
	t *testing.T,
	successArgs []string,
	failureArgs []string,
	malformed bool,
) mutationFixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	statePath := config.IdentityStatePath()
	if malformed {
		if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(statePath, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return mutationFixture{
		args: successArgs, failureArgs: failureArgs,
		snapshot:      fileSnapshot(statePath),
		assertSuccess: changedFileAssertion("identity state"),
	}
}

func newIdentitySyncMutationFixture(t *testing.T, endpoint string, success bool) mutationFixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	serverPath := filepath.Join(root, layout.ServerConfigFileName)
	content := `[server]
install_dir = "` + safeServerInstallDir() + `"
host = "127.0.0.1"

[server.identity_provider]
instance_id = "provider-a"
kind = "scim"
secret = "provider-auth-value"

[server.identity_provider.scim]
endpoint = "` + endpoint + `"
timeout = "2s"
`
	if err := os.WriteFile(serverPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot := fileSnapshot(serverPath)
	assert := changedFileAssertion("identity Desired")
	if success {
		snapshot = fileSnapshot(config.IdentityStatePath())
		assert = changedFileAssertion("identity state")
	}
	return mutationFixture{
		args:          []string{"server", "identity", "sync"},
		failureArgs:   []string{"server", "identity", "sync"},
		sensitive:     []string{"provider-auth-value"},
		snapshot:      snapshot,
		assertSuccess: assert,
	}
}

func fileSnapshot(path string) func(*testing.T) any {
	return func(t *testing.T) any {
		t.Helper()
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
}

func changedFileAssertion(name string) func(*testing.T, any, any) {
	return func(t *testing.T, before, after any) {
		t.Helper()
		if before == after {
			t.Fatalf("%s was not updated", name)
		}
	}
}
