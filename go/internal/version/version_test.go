package version

import "testing"

func TestCurrentReturnsDefault(t *testing.T) {
	if got := Current(); got == "" {
		t.Fatalf("Current returned empty string")
	}
}

func TestAtLeast(t *testing.T) {
	tests := []struct {
		current string
		minimum string
		want    bool
	}{
		{current: "0.2.6", minimum: "0.2.8", want: false},
		{current: "0.2.8", minimum: "0.2.8", want: true},
		{current: "0.2.9", minimum: "0.2.8", want: true},
		{current: "v0.3.0", minimum: "0.2.8", want: true},
		{current: "0.2.8-dev", minimum: "0.2.8", want: true},
	}

	for _, tc := range tests {
		if got := AtLeast(tc.current, tc.minimum); got != tc.want {
			t.Fatalf("AtLeast(%q, %q) = %v, want %v", tc.current, tc.minimum, got, tc.want)
		}
	}
}
