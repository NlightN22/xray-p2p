package config

import "testing"

func TestLoadInvalidPath(t *testing.T) {
	dir := chdirTemp(t)

	_, err := Load(Options{Path: dir})
	if err == nil {
		t.Fatalf("expected error for directory path")
	}
}
