package netutil

import "testing"

func TestValidateHost(t *testing.T) {
	valid := []string{
		"10.0.0.1",
		"2001:db8::1",
		"example.com",
		"sub.example.co.uk",
		"EXAMPLE.org",
		"host-with-dash.example",
		"example.com.",
	}
	for _, value := range valid {
		if err := ValidateHost(value); err != nil {
			t.Fatalf("ValidateHost(%q): %v", value, err)
		}
		if !IsValidHost(value) {
			t.Fatalf("IsValidHost(%q) returned false", value)
		}
	}

	invalid := []string{
		"",
		"   ",
		".",
		"-example.com",
		"example-.com",
		"exa mple.com",
		"example..com",
		"example-.com",
		"256.0.0.1",
		"host*example.com",
		"example.com..",
		"-",
	}
	for _, value := range invalid {
		if err := ValidateHost(value); err == nil {
			t.Fatalf("ValidateHost(%q) expected error", value)
		}
		if IsValidHost(value) {
			t.Fatalf("IsValidHost(%q) returned true", value)
		}
	}
}
