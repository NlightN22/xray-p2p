package version

// current holds the application version. It is overridden at build time via
// -ldflags "-X github.com/NlightN22/xray-p2p/go/internal/version.current=...".
var current = "0.1.0"

// Current returns the application version string embedded at build time.
func Current() string {
	return current
}
