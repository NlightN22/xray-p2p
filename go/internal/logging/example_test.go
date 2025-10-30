package logging_test

import (
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func Example() {
	// Increase verbosity when diagnosing issues.
	logging.SetLevel("debug")

	// Attach shared attributes (e.g. trace identifiers) to related log lines.
	logger := logging.With("trace_id", "abc123")
	logger.Info("dial attempt scheduled", "target", "gateway")

	// Convenience helpers are available for the common levels.
	logging.Debug("listener ready", "proto", "tcp")
}
