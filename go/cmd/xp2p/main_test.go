package main

import "testing"

func TestJSONOutputRequested(t *testing.T) {
	for _, args := range [][]string{{"--json"}, {"client", "-J", "list"}, {"--json=true"}} {
		if !jsonOutputRequested(args) {
			t.Fatalf("expected JSON for %v", args)
		}
	}
	for _, args := range [][]string{nil, {"--log-json"}, {"--json=false"}} {
		if jsonOutputRequested(args) {
			t.Fatalf("did not expect JSON for %v", args)
		}
	}
}
