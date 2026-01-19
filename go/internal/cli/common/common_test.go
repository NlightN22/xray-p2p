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

func TestValidateRFC3986Unreserved(t *testing.T) {
	if err := ValidateRFC3986Unreserved("Az09-._~"); err != nil {
		t.Fatalf("expected valid value, got %v", err)
	}
	if err := ValidateRFC3986Unreserved("a b"); err == nil {
		t.Fatalf("expected invalid value with space")
	}
	if err := ValidateRFC3986Unreserved("bad+plus"); err == nil {
		t.Fatalf("expected invalid value with plus")
	}
}
