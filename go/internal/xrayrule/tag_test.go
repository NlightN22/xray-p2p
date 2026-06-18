package xrayrule

import "testing"

func TestRedirectTagIsStableAndNormalized(t *testing.T) {
	left := Redirect("client", " Proxy-Alpha ", "DOMAIN", "App.Example")
	right := Redirect(" client ", "proxy-alpha", "domain", "app.example")
	if left != right {
		t.Fatalf("tags differ: %q != %q", left, right)
	}
	if left == Redirect("server", "proxy-alpha", "domain", "app.example") {
		t.Fatalf("role should affect tag")
	}
}

func TestDifferentRuleKindsDoNotCollide(t *testing.T) {
	redirect := Redirect("client", "proxy-alpha", "domain", "app.example")
	marker := DiagnosticsMarker("client", "proxy-alpha")
	if redirect == marker {
		t.Fatalf("different rule kinds produced same tag %q", redirect)
	}
}
