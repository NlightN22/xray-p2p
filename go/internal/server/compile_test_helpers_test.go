//go:build windows || linux

package server

import (
	"encoding/json"
	"testing"
)

func compileDesiredDoc(t *testing.T, configPath string, extensionsDir string) map[string]any {
	t.Helper()
	artifacts, err := compileDesired(configPath, extensionsDir)
	if err != nil {
		t.Fatalf("compile desired: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(artifacts.XrayJSON, &doc); err != nil {
		t.Fatalf("parse xray.json: %v", err)
	}
	return doc
}

func extractRoutingRules(t *testing.T, doc map[string]any) []any {
	t.Helper()
	routing, ok := doc["routing"].(map[string]any)
	if !ok {
		t.Fatalf("expected routing section, got %T", doc["routing"])
	}
	rules, ok := routing["rules"].([]any)
	if !ok {
		t.Fatalf("expected routing.rules array, got %T", routing["rules"])
	}
	return rules
}
