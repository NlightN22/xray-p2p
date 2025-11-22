package version

import "testing"

func TestCurrentReturnsDefault(t *testing.T) {
	if got := Current(); got == "" {
		t.Fatalf("Current returned empty string")
	}
}
