package xrayapi

import "testing"

func TestAPIListenFromConfigUsesLegacyAPIListen(t *testing.T) {
	got, err := APIListenFromConfig([]byte(`{"api":{"listen":"127.0.0.1:10085"}}`))
	if err != nil {
		t.Fatalf("APIListenFromConfig: %v", err)
	}
	if got != "127.0.0.1:10085" {
		t.Fatalf("address = %q", got)
	}
}

func TestAPIListenFromConfigUsesAPIInbound(t *testing.T) {
	got, err := APIListenFromConfig([]byte(`{
		"api":{"tag":"api"},
		"inbounds":[{"tag":"api","listen":"127.0.0.1","port":10085}]
	}`))
	if err != nil {
		t.Fatalf("APIListenFromConfig: %v", err)
	}
	if got != "127.0.0.1:10085" {
		t.Fatalf("address = %q", got)
	}
}
