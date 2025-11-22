package netutil

import (
	"errors"
	"strings"
	"testing"
)

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

func TestValidateHostLengthAndUnicodeErrors(t *testing.T) {
	t.Parallel()

	longHost := strings.Repeat("a", 254) + ".example.com"
	if err := ValidateHost(longHost); !errors.Is(err, ErrHostTooLong) {
		t.Fatalf("expected ErrHostTooLong, got %v", err)
	}

	longLabel := strings.Repeat("b", 64) + ".example.com"
	if err := ValidateHost(longLabel); err == nil || !strings.Contains(err.Error(), "exceeds 63") {
		t.Fatalf("expected label length error, got %v", err)
	}

	unicodeHost := "ümlaut.example.com"
	if err := ValidateHost(unicodeHost); err == nil || !strings.Contains(err.Error(), "non-ASCII") {
		t.Fatalf("expected unicode error, got %v", err)
	}

	fakeIPv4 := "123.456.789.0"
	if err := ValidateHost(fakeIPv4); err == nil || !strings.Contains(err.Error(), "invalid IPv4") {
		t.Fatalf("expected invalid IPv4 error, got %v", err)
	}
}
