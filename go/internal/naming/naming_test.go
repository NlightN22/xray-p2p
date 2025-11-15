package naming

import "testing"

func TestSanitizeLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Example.COM", "example-com"},
		{"  Mixed--Value  ", "mixed-value"},
		{"UPPER_lower-123", "upper-lower-123"},
		{"Symbols!*&", "symbols"},
		{"..dots..and::colons", "dots-and-colons"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := SanitizeLabel(tt.input); got != tt.want {
			t.Fatalf("SanitizeLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestReverseTag(t *testing.T) {
	tag, err := ReverseTag("User.Name", "Edge.EXAMPLE.com")
	if err != nil {
		t.Fatalf("ReverseTag returned error: %v", err)
	}
	if tag != "user-nameedge-example-com.rev" {
		t.Fatalf("ReverseTag = %q, want user-nameedge-example-com.rev", tag)
	}

	if _, err := ReverseTag("  !!!  ", "host"); err == nil {
		t.Fatalf("expected error for invalid user id")
	}
	if _, err := ReverseTag("user", "   "); err == nil {
		t.Fatalf("expected error for invalid user id")
	}
}
