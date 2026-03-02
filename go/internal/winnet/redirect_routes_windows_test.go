//go:build windows

package winnet

import "testing"

func TestNormalizeCIDRs(t *testing.T) {
	got := normalizeCIDRs([]string{" 10.0.0.0/24 ", "10.0.0.0/24", " ", "10.0.1.0/24"})
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %#v", len(got), got)
	}
	if got[0] != "10.0.0.0/24" || got[1] != "10.0.1.0/24" {
		t.Fatalf("unexpected order/values: %#v", got)
	}
}

func TestParseTunAddr(t *testing.T) {
	ip, prefix, ok := parseTunAddr("198.18.0.1/30")
	if !ok {
		t.Fatalf("expected ok")
	}
	if ip != "198.18.0.1" || prefix != 30 {
		t.Fatalf("unexpected result: ip=%s prefix=%d ok=%t", ip, prefix, ok)
	}

	ip, prefix, ok = parseTunAddr("2001:db8::1/64")
	if ok || ip != "" || prefix != 0 {
		t.Fatalf("expected ipv6 to be rejected: ip=%s prefix=%d ok=%t", ip, prefix, ok)
	}
}

func TestBuildPowerShellArray(t *testing.T) {
	got := buildPowerShellArray([]string{"10.0.0.0/24", "10.0.1.0/24"})
	if got != "@('10.0.0.0/24','10.0.1.0/24')" {
		t.Fatalf("unexpected array: %s", got)
	}
	if buildPowerShellArray(nil) != "@()" {
		t.Fatalf("expected empty array for nil")
	}
}
