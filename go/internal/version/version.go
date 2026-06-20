package version

import (
	"strconv"
	"strings"
)

// current holds the application version. It is overridden at build time via
// -ldflags "-X github.com/NlightN22/xray-p2p/go/internal/version.current=...".
var current = "0.2.7"

// Current returns the application version string embedded at build time.
func Current() string {
	return current
}

// AtLeast reports whether current is greater than or equal to minimum.
func AtLeast(current, minimum string) bool {
	currentParts := parseParts(current)
	minimumParts := parseParts(minimum)
	for i := 0; i < 3; i++ {
		if currentParts[i] > minimumParts[i] {
			return true
		}
		if currentParts[i] < minimumParts[i] {
			return false
		}
	}
	return true
}

func parseParts(value string) [3]int {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	value = strings.SplitN(value, "-", 2)[0]
	parts := strings.Split(value, ".")
	var parsed [3]int
	for i := 0; i < len(parts) && i < len(parsed); i++ {
		number, err := strconv.Atoi(parts[i])
		if err != nil {
			return [3]int{}
		}
		parsed[i] = number
	}
	return parsed
}
