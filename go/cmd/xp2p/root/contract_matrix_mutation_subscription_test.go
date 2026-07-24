package root

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func registerClientSubscriptionMutationContracts(registry map[string]mutationContract) {
	registerMutation(
		registry,
		"xp2p client subscription add",
		func(t *testing.T) mutationFixture {
			server := newSubscriptionFixtureServer(t, false, "first-subscription-value")
			return newSubscriptionMutationFixture(t,
				[]string{"client", "subscription", "add", "matrix", server.URL, "--allow-http"},
				[]string{"client", "subscription", "add", "matrix", server.URL, "--allow-http"},
			)
		},
		func(t *testing.T) mutationFixture {
			server := newSubscriptionFixtureServer(t, true, "")
			return newSubscriptionMutationFixture(t,
				[]string{"client", "subscription", "add", "matrix", server.URL, "--allow-http"},
				[]string{"client", "subscription", "add", "matrix", server.URL, "--allow-http"},
			)
		},
	)
	registerMutation(
		registry,
		"xp2p client subscription refresh",
		func(t *testing.T) mutationFixture {
			body := "first-subscription-value"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(subscriptionTrojanLink(body)))
			}))
			t.Cleanup(server.Close)
			fixture := newSubscriptionMutationFixture(t,
				[]string{"client", "subscription", "refresh", "matrix", "--allow-http"},
				[]string{"client", "subscription", "refresh", "missing", "--allow-http"},
			)
			mustRunMutationPrerequisite(t,
				[]string{"client", "subscription", "add", "matrix", server.URL, "--allow-http"})
			body = "refreshed-subscription-value"
			return fixture
		},
		func(t *testing.T) mutationFixture {
			server := newSubscriptionFixtureServer(t, false, "first-subscription-value")
			fixture := newSubscriptionMutationFixture(t,
				[]string{"client", "subscription", "refresh", "matrix", "--allow-http"},
				[]string{"client", "subscription", "refresh", "missing", "--allow-http"},
			)
			mustRunMutationPrerequisite(t,
				[]string{"client", "subscription", "add", "matrix", server.URL, "--allow-http"})
			return fixture
		},
	)
	registerMutation(
		registry,
		"xp2p client subscription remove",
		func(t *testing.T) mutationFixture {
			server := newSubscriptionFixtureServer(t, false, "first-subscription-value")
			fixture := newSubscriptionMutationFixture(t,
				[]string{"client", "subscription", "remove", "matrix"},
				[]string{"client", "subscription", "remove", "missing"},
			)
			mustRunMutationPrerequisite(t,
				[]string{"client", "subscription", "add", "matrix", server.URL, "--allow-http"})
			return fixture
		},
		func(t *testing.T) mutationFixture {
			return newSubscriptionMutationFixture(t,
				[]string{"client", "subscription", "remove", "matrix"},
				[]string{"client", "subscription", "remove", "missing"},
			)
		},
	)
}

func newSubscriptionFixtureServer(t *testing.T, fail bool, password string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail {
			http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(subscriptionTrojanLink(password)))
	}))
	t.Cleanup(server.Close)
	return server
}

func subscriptionTrojanLink(password string) string {
	return "trojan://" + password + "@127.0.0.1:443?security=tls&type=tcp&sni=matrix.example#Matrix"
}

func newSubscriptionMutationFixture(
	t *testing.T,
	successArgs []string,
	failureArgs []string,
) mutationFixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	path := filepath.Join(root, layout.ClientConfigFileName)
	return mutationFixture{
		args: successArgs, failureArgs: failureArgs,
		sensitive: []string{"first-subscription-value", "refreshed-subscription-value"},
		snapshot: func(t *testing.T) any {
			t.Helper()
			data, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				return ""
			}
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		},
		assertSuccess: func(t *testing.T, before, after any) {
			t.Helper()
			if before == after {
				t.Fatal("subscription mutation did not update Desired")
			}
		},
	}
}
