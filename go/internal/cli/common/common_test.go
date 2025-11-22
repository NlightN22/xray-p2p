package common

import (
	"os"
	"testing"
)

func TestFirstNonEmpty(t *testing.T) {
	if val := FirstNonEmpty(" ", "\t", "value", "other"); val != "value" {
		t.Fatalf("FirstNonEmpty returned %q", val)
	}
	if FirstNonEmpty(" ", "") != "" {
		t.Fatalf("FirstNonEmpty should return empty when all inputs empty")
	}
}

func TestPromptYesNo(t *testing.T) {
	old := os.Stdin
	defer func() { os.Stdin = old }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.WriteString("y\n"); err != nil {
		t.Fatalf("write pipe: %v", err)
	}
	w.Close()
	os.Stdin = r
	if ok, err := PromptYesNo("Test question?"); err != nil || !ok {
		t.Fatalf("PromptYesNo expected yes, got %v %v", ok, err)
	}
}
