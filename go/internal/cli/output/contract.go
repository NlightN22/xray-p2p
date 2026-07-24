package output

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"

	"github.com/spf13/cobra"
)

const (
	AnnotationClass  = "xp2p.output.class"
	AnnotationReason = "xp2p.output.reason"

	ClassJSON      = "json"
	ClassGenerator = "generator"
	ClassLifecycle = "lifecycle"
	ClassStreaming = "streaming"

	SchemaVersion = "1"
)

// Envelope is the stable top-level contract for successful CLI JSON output.
type Envelope struct {
	SchemaVersion string `json:"schema_version"`
	Command       string `json:"command"`
	Result        any    `json:"result"`
}

// ErrorEnvelope is the stable top-level contract for failed CLI JSON output.
type ErrorEnvelope struct {
	SchemaVersion string      `json:"schema_version"`
	Command       string      `json:"command"`
	Error         ErrorDetail `json:"error"`
}

// ErrorDetail contains a machine-readable code and a safe diagnostic.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OperationResult is the common result for successful mutations without payload.
type OperationResult struct {
	Status string `json:"status"`
}

type resultCollector struct {
	value any
}

type collectorContextKey struct{}

// Enabled reports whether the command is running under the JSON presenter.
func Enabled(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd.Context().Value(collectorContextKey{}).(*resultCollector)
	return ok
}

// EnabledContext reports whether a context contains the JSON result collector.
func EnabledContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Value(collectorContextKey{}).(*resultCollector)
	return ok
}

// SetResult publishes a typed result for the JSON presentation layer.
func SetResult(cmd *cobra.Command, value any) error {
	if cmd == nil {
		return errors.New("command is nil")
	}
	return SetResultContext(cmd.Context(), value)
}

// SetResultContext publishes a typed result from a use-case adapter.
func SetResultContext(ctx context.Context, value any) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	collector, ok := ctx.Value(collectorContextKey{}).(*resultCollector)
	if !ok {
		return nil
	}
	if value == nil {
		return errors.New("JSON result must not be nil")
	}
	collector.value = value
	return nil
}

// RenderedError prevents the process entry point from printing a second error.
type RenderedError struct {
	err error
}

func (e RenderedError) Error() string  { return e.err.Error() }
func (e RenderedError) Unwrap() error  { return e.err }
func (e RenderedError) Rendered() bool { return true }

// MarkRendered identifies an error whose JSON representation was already emitted.
func MarkRendered(err error) error {
	return RenderedError{err: err}
}

// Classify records the output contract of a command.
func Classify(cmd *cobra.Command, class, reason string) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[AnnotationClass] = class
	if reason != "" {
		cmd.Annotations[AnnotationReason] = reason
	}
}

// Class returns the declared output class.
func Class(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	return cmd.Annotations[AnnotationClass]
}

// WriteError writes exactly one JSON error document.
func WriteError(w io.Writer, command, code string, err error) error {
	if err == nil {
		err = errors.New("command failed")
	}
	return json.NewEncoder(w).Encode(ErrorEnvelope{
		SchemaVersion: SchemaVersion,
		Command:       command,
		Error: ErrorDetail{
			Code:    code,
			Message: redactSensitive(err.Error()),
		},
	})
}

var (
	credentialURLPattern = regexp.MustCompile(`(?i)\b(trojan|vless)://[^@\s]+@`)
	credentialKVPattern  = regexp.MustCompile(`(?i)\b(password|token|secret|credential)(\s*[=:]\s*)\S+`)
	credentialArgPattern = regexp.MustCompile(`(?i)(--(?:password|token|secret|credential)\s+)\S+`)
)

func redactSensitive(message string) string {
	message = credentialURLPattern.ReplaceAllString(message, "$1://REDACTED@")
	message = credentialKVPattern.ReplaceAllString(message, "$1$2REDACTED")
	return credentialArgPattern.ReplaceAllString(message, "$1REDACTED")
}

// WrapJSON decorates a leaf command without changing its human-readable path.
func WrapJSON(cmd *cobra.Command, enabled func() bool, defaultOperation bool) {
	if cmd == nil || Class(cmd) != ClassJSON {
		return
	}
	runE := cmd.RunE
	run := cmd.Run
	if runE == nil && run == nil {
		return
	}
	cmd.Run = nil
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if !enabled() {
			if runE != nil {
				return runE(cmd, args)
			}
			run(cmd, args)
			return nil
		}
		collector := &resultCollector{}
		originalContext := cmd.Context()
		cmd.SetContext(context.WithValue(originalContext, collectorContextKey{}, collector))
		defer cmd.SetContext(originalContext)
		_, err := captureStdout(cmd, func() error {
			if runE != nil {
				return runE(cmd, args)
			}
			run(cmd, args)
			return nil
		})
		if err != nil {
			if writeErr := WriteError(cmd.ErrOrStderr(), cmd.CommandPath(), "command_failed", err); writeErr != nil {
				return writeErr
			}
			return RenderedError{err: err}
		}
		if collector.value == nil {
			if defaultOperation {
				collector.value = OperationResult{Status: "completed"}
			} else {
				err := errors.New("command did not publish its typed JSON result")
				if writeErr := WriteError(cmd.ErrOrStderr(), cmd.CommandPath(), "missing_json_result", err); writeErr != nil {
					return writeErr
				}
				return RenderedError{err: err}
			}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(Envelope{
			SchemaVersion: SchemaVersion,
			Command:       cmd.CommandPath(),
			Result:        collector.value,
		})
	}
}

// RejectJSON prevents standalone streams and generators from being corrupted.
func RejectJSON(cmd *cobra.Command, enabled func() bool) {
	if cmd == nil || Class(cmd) == ClassJSON {
		return
	}
	runE := cmd.RunE
	run := cmd.Run
	if runE == nil && run == nil {
		return
	}
	cmd.Run = nil
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if enabled() {
			reason := cmd.Annotations[AnnotationReason]
			err := fmt.Errorf("JSON output is not available: %s", reason)
			if writeErr := WriteError(cmd.ErrOrStderr(), cmd.CommandPath(), "unsupported_output_format", err); writeErr != nil {
				return writeErr
			}
			return RenderedError{err: err}
		}
		if runE != nil {
			return runE(cmd, args)
		}
		run(cmd, args)
		return nil
	}
}

var captureMu sync.Mutex

func captureStdout(cmd *cobra.Command, execute func() error) ([]byte, error) {
	captureMu.Lock()
	defer captureMu.Unlock()

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	originalStdin := os.Stdin
	originalOut := cmd.OutOrStdout()
	originalErr := cmd.ErrOrStderr()
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		_ = reader.Close()
		_ = writer.Close()
		return nil, err
	}
	_ = stdinWriter.Close()
	errReader, errWriter, err := os.Pipe()
	if err != nil {
		_ = reader.Close()
		_ = writer.Close()
		_ = stdinReader.Close()
		return nil, err
	}
	var buffer bytes.Buffer
	copyDone := make(chan error, 1)
	errCopyDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&buffer, reader)
		copyDone <- copyErr
	}()
	go func() {
		_, copyErr := io.Copy(io.Discard, errReader)
		errCopyDone <- copyErr
	}()

	os.Stdout = writer
	os.Stderr = errWriter
	os.Stdin = stdinReader
	cmd.SetOut(writer)
	cmd.SetErr(errWriter)
	runErr := execute()
	_ = writer.Close()
	_ = errWriter.Close()
	os.Stdout = originalStdout
	os.Stderr = originalStderr
	os.Stdin = originalStdin
	cmd.SetOut(originalOut)
	cmd.SetErr(originalErr)
	copyErr := <-copyDone
	errCopyErr := <-errCopyDone
	_ = reader.Close()
	_ = errReader.Close()
	_ = stdinReader.Close()
	if runErr != nil {
		return buffer.Bytes(), runErr
	}
	if copyErr != nil {
		return buffer.Bytes(), copyErr
	}
	return buffer.Bytes(), errCopyErr
}
