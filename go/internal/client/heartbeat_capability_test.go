package client

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
)

func TestDiscoverHeartbeatCapability(t *testing.T) {
	for _, test := range []struct {
		name         string
		capabilities []string
		want         heartbeat.Capability
	}{
		{name: "diagnostics sidecar", capabilities: []string{"xp2p-diag"}, want: heartbeat.CapabilityXP2PDiag},
		{name: "legacy full server", want: heartbeat.CapabilityXP2PHeartbeat},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != controlplane.PathReady {
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ready":        true,
					"capabilities": test.capabilities,
				})
			}))
			defer server.Close()
			parsed, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			host, portText, err := net.SplitHostPort(parsed.Host)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.Atoi(portText)
			if err != nil {
				t.Fatal(err)
			}
			got, err := discoverHeartbeatCapability(context.Background(), host, port, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("capability = %q, want %q", got, test.want)
			}
		})
	}
}
