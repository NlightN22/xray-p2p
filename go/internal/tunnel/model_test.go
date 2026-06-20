package tunnel

import (
	"strings"
	"testing"
)

func TestNormalizeLegacyEndpointUsesTrojanTLS(t *testing.T) {
	endpoint, err := Normalize(Endpoint{Host: "edge.example", Port: 443})
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.Profile != ProfileTrojanTLS || endpoint.Protocol != "trojan" || endpoint.Security != "tls" {
		t.Fatalf("unexpected legacy mapping: %#v", endpoint)
	}
}

func TestLinkRoundTripPreservesUnknownParameters(t *testing.T) {
	parsed, err := ParseLink("trojan://secret@edge.example:443?security=tls&sni=edge.example&foo=bar#alice")
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderLink(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "foo=bar") || !strings.Contains(rendered, "#alice") {
		t.Fatalf("round trip lost values: %s", rendered)
	}
}

func TestVLESSLinkMapping(t *testing.T) {
	link, err := ParseLink("vless://550e8400-e29b-41d4-a716-446655440000@edge.example:443?security=tls&type=tcp&flow=xtls-rprx-vision#alice")
	if err != nil {
		t.Fatal(err)
	}
	if link.User.Credential != "550e8400-e29b-41d4-a716-446655440000" || link.User.UserLabel != "alice" {
		t.Fatalf("unexpected link: %#v", link)
	}
	fragment, err := XrayInboundUser("vless", link.User)
	if err != nil || fragment["id"] != link.User.Credential {
		t.Fatalf("unexpected VLESS mapping: %#v, %v", fragment, err)
	}
}

func TestNewCredentialReturnsUUID(t *testing.T) {
	credential, err := NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	if len(credential) != 36 || credential[14] != '4' {
		t.Fatalf("not UUID v4: %q", credential)
	}
}

func TestNormalizeRecordMapsLegacyTrojanFields(t *testing.T) {
	record, report, err := NormalizeRecord(LegacyRecord{Host: "edge.example", Port: 443, UserLabel: "alice", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if record.Endpoint.Profile != ProfileTrojanTLS || record.User.Credential != "secret" || record.User.UserLabel != "alice" {
		t.Fatalf("unexpected record: %#v", record)
	}
	if len(report.AppliedRules) != 1 {
		t.Fatalf("expected compatibility rule, got %#v", report)
	}
}

func TestNormalizeRecordRejectsLegacyFieldsAfterRemoval(t *testing.T) {
	previous := currentAppVersion
	currentAppVersion = func() string { return "0.2.8" }
	defer func() { currentAppVersion = previous }()
	_, _, err := NormalizeRecord(LegacyRecord{Host: "edge.example", Port: 443, Password: "secret"})
	if err == nil || !strings.Contains(err.Error(), "removed legacy fields") {
		t.Fatalf("expected removed-field error, got %v", err)
	}
}
