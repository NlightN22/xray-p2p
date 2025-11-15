package naming

import (
	"fmt"
	"strings"
)

const reverseSuffix = ".rev"

// SanitizeLabel normalizes identifiers used in tags and routing rules.
func SanitizeLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// ReverseTag builds the shared reverse-tunnel identifier for a user and host combination.
func ReverseTag(userID, host string) (string, error) {
	user := SanitizeLabel(userID)
	hostLabel := SanitizeLabel(host)
	if user == "" || hostLabel == "" {
		return "", fmt.Errorf("xp2p: unable to derive reverse identifier from %q/%q", strings.TrimSpace(userID), strings.TrimSpace(host))
	}
	return user + hostLabel + reverseSuffix, nil
}
