//go:build linux

package linuxnet

import "testing"

func TestBuildNetworkdConfigIncludesDefaults(t *testing.T) {
	cfg := buildNetworkdConfig("xp2pc", "198.18.0.1/30", 1500, 20090)
	if !containsLine(cfg, "# "+managedMarker) {
		t.Fatalf("expected managed marker")
	}
	if !containsLine(cfg, "Name = xp2pc") {
		t.Fatalf("expected match name")
	}
	if !containsLine(cfg, "Address = 198.18.0.1/30") {
		t.Fatalf("expected address line")
	}
	if !containsLine(cfg, "MTUBytes = 1500") {
		t.Fatalf("expected mtu line")
	}
	if !containsLine(cfg, "Table = 20090") {
		t.Fatalf("expected route table line")
	}
	if !containsLine(cfg, "Destination = 0.0.0.0/0") {
		t.Fatalf("expected default route destination")
	}
	if containsLine(cfg, "RoutingPolicyRule") {
		t.Fatalf("did not expect routing policy rule block")
	}
}

func TestBuildNetworkdConfigOmitsMTUWhenZero(t *testing.T) {
	cfg := buildNetworkdConfig("xp2ps", "198.18.0.5/30", 0, 20091)
	if containsLine(cfg, "MTUBytes =") {
		t.Fatalf("unexpected mtu line")
	}
}

func TestTableForNameDefaults(t *testing.T) {
	if tableForName("xp2pc") != 20090 {
		t.Fatalf("expected xp2pc table 20090")
	}
	if tableForName("xp2ps") != 20091 {
		t.Fatalf("expected xp2ps table 20091")
	}
	if tableForName("custom0") != 20090 {
		t.Fatalf("expected default table 20090")
	}
}

func containsLine(cfg, line string) bool {
	for _, ln := range splitLines(cfg) {
		if ln == line {
			return true
		}
	}
	return false
}

func splitLines(cfg string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(cfg); i++ {
		if cfg[i] == '\n' {
			lines = append(lines, trimTrailingCR(cfg[start:i]))
			start = i + 1
		}
	}
	if start <= len(cfg) {
		lines = append(lines, trimTrailingCR(cfg[start:]))
	}
	return lines
}

func trimTrailingCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}
