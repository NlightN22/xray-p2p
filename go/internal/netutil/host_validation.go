package netutil

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"unicode"
)

var (
	// ErrHostEmpty indicates that the host value is empty after trimming.
	ErrHostEmpty = errors.New("netutil: host is empty")
	// ErrHostTooLong indicates that the host value exceeds the maximum DNS length.
	ErrHostTooLong = errors.New("netutil: host exceeds 253 characters")
)

// ValidateHost checks whether the provided value is a valid IPv4/IPv6 address or a DNS hostname.
// It returns nil when the host is valid, otherwise an error describing the problem.
func ValidateHost(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ErrHostEmpty
	}
	if net.ParseIP(trimmed) != nil {
		return nil
	}
	if len(trimmed) > 253 {
		return ErrHostTooLong
	}
	if looksLikeIPv4(trimmed) {
		return fmt.Errorf("netutil: invalid IPv4 address %q", value)
	}

	host := trimmed
	// Allow trailing dot for fully qualified names.
	if strings.HasSuffix(host, ".") {
		host = host[:len(host)-1]
		if host == "" {
			return fmt.Errorf("netutil: invalid host %q", value)
		}
	}

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if err := validateHostLabel(label); err != nil {
			return fmt.Errorf("netutil: invalid host %q: %w", value, err)
		}
	}
	return nil
}

// IsValidHost reports whether the string is a valid host.
func IsValidHost(value string) bool {
	return ValidateHost(value) == nil
}

func validateHostLabel(label string) error {
	if label == "" {
		return errors.New("empty label")
	}
	if len(label) > 63 {
		return fmt.Errorf("label %q exceeds 63 characters", label)
	}
	for i, r := range label {
		if r > unicode.MaxASCII {
			return fmt.Errorf("label %q contains non-ASCII characters", label)
		}
		if !isLabelChar(r) {
			return fmt.Errorf("label %q contains invalid character %q", label, r)
		}
		if (i == 0 || i == len(label)-1) && r == '-' {
			return fmt.Errorf("label %q must not start or end with '-'", label)
		}
	}
	return nil
}

func isLabelChar(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '-'
}

func looksLikeIPv4(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if !unicode.IsDigit(r) {
				return false
			}
		}
	}
	return true
}
