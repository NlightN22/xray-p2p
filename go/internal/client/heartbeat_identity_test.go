package client

import "testing"

func TestHeartbeatEndpointIDCoversDiagnosticRoute(t *testing.T) {
	base := clientEndpointRecord{
		Tag: "proxy-edge", User: "alice", Hostname: "edge.example", Address: "192.0.2.10",
		Port: 443, Profile: "trojan-tls", Protocol: "trojan", Transport: "tcp", Security: "tls",
		ServerName: "edge.example", PinnedPeerCertSHA256: "aabb",
	}
	want := heartbeatEndpointID(base, "198.18.0.2", DiagnosticsMarkerPort)
	cases := []clientEndpointRecord{base, base, base, base}
	cases[0].Address = "192.0.2.11"
	cases[1].Port = 8443
	cases[2].ServerName = "other.example"
	cases[3].PinnedPeerCertSHA256 = "ccdd"
	for _, changed := range cases {
		if got := heartbeatEndpointID(changed, "198.18.0.2", DiagnosticsMarkerPort); got == want {
			t.Fatalf("route change did not change endpoint ID: %+v", changed)
		}
	}
	if got := heartbeatEndpointID(base, "198.18.0.3", DiagnosticsMarkerPort); got == want {
		t.Fatal("diagnostics target change did not change endpoint ID")
	}
	if got := heartbeatEndpointID(base, "198.18.0.2", DiagnosticsMarkerPort+1); got == want {
		t.Fatal("diagnostics port change did not change endpoint ID")
	}
}
