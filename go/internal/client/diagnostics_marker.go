package client

import "fmt"

const (
	DiagnosticsMarkerPort = 62022
	diagnosticsMarkerMax  = 65535
)

func markerIPForIndex(index int) (string, error) {
	if index < 0 || index >= diagnosticsMarkerMax {
		return "", fmt.Errorf("marker index %d is out of range", index)
	}
	value := index + 1
	octet2 := value / 256
	octet3 := value % 256
	return fmt.Sprintf("127.255.%d.%d", octet2, octet3), nil
}
