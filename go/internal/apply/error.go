package apply

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

type ErrorMarker struct {
	RequestID string    `json:"request_id"`
	Role      string    `json:"role"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
}

func WriteError(path string, marker ErrorMarker, auditPath string) error {
	marker.RequestID = strings.TrimSpace(marker.RequestID)
	marker.Role = strings.TrimSpace(strings.ToLower(marker.Role))
	marker.Reason = sanitizeReason(marker.Reason)
	if marker.Timestamp.IsZero() {
		marker.Timestamp = time.Now().UTC()
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("apply: encode error marker: %w", err)
	}
	data = append(data, '\n')
	return configio.WriteBytes(path, data, configio.WriteOptions{
		AuditPath:         auditPath,
		IgnoreAuditErrors: true,
	})
}

func ReadError(path string) (ErrorMarker, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrorMarker{}, false, nil
		}
		return ErrorMarker{}, false, fmt.Errorf("apply: read error marker: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return ErrorMarker{}, true, nil
	}
	var marker ErrorMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		trimmed := strings.TrimSpace(string(data))
		repaired := strings.ReplaceAll(trimmed, "\\r\\n", "\n")
		repaired = strings.ReplaceAll(repaired, "\\n", "\n")
		if retryErr := json.Unmarshal([]byte(repaired), &marker); retryErr != nil {
			return ErrorMarker{}, true, fmt.Errorf("apply: parse error marker: %w", err)
		}
	}
	marker.RequestID = strings.TrimSpace(marker.RequestID)
	marker.Role = strings.TrimSpace(strings.ToLower(marker.Role))
	marker.Reason = strings.TrimSpace(marker.Reason)
	return marker, true, nil
}

func RemoveError(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("apply: remove error marker: %w", err)
	}
	return nil
}

func sanitizeReason(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\r\n", "\n")
	trimmed = strings.ReplaceAll(trimmed, "\r", "\n")
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	return strings.TrimSpace(trimmed)
}
