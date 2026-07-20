package configio

import (
	"strings"
	"testing"
)

func TestAuditCommandRedactsSubscriptionAndConnectionSecrets(t *testing.T) {
	command := auditCommand([]string{"xp2p", "client", "subscription", "add", "main", "https://panel.example/sub/secret-id?token=value", "--link", "trojan://password@edge.example:443#user"})
	for _, secret := range []string{"secret-id", "token=value", "password", "#user"} {
		if strings.Contains(command, secret) {
			t.Fatalf("audit command leaked %q: %s", secret, command)
		}
	}
}
