package logging

import (
	"io"
	"strings"
)

// Options controls logger configuration.
type Options struct {
	// Level is a textual representation of the desired log level (debug/info/warn/error).
	// When empty, the current level is preserved.
	Level string
	// Output selects the destination for log records. When nil, os.Stderr is used.
	Output io.Writer
	// Format selects the output format ("text" or "json"). Empty value keeps current format.
	Format Format
}

// Format describes the logger output format.
type Format string

const (
	// FormatText outputs human-readable log lines.
	FormatText Format = "text"
	// FormatJSON outputs structured JSON records.
	FormatJSON Format = "json"
)

// Configure allows the caller to adjust the global logger at runtime.
func Configure(opts Options) {
	if strings.TrimSpace(opts.Level) != "" {
		levelVar.Set(parseLevel(opts.Level))
	}
	if opts.Format != "" {
		switch opts.Format {
		case FormatJSON:
			formatVar.Store(FormatJSON)
		default:
			formatVar.Store(FormatText)
		}
	}
	setLogger(opts.Output)
}
