package client

import (
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/xrayrule"
)

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

func diagnosticsMarkerRules(endpoints []clientEndpointRecord) ([]any, error) {
	rules := make([]any, 0, len(endpoints))
	for idx, ep := range endpoints {
		if ep.Disabled {
			continue
		}
		markerIP, err := markerIPForIndex(idx)
		if err != nil {
			return nil, fmt.Errorf("allocate diagnostics marker for %s: %w", ep.Tag, err)
		}
		rules = append(rules, map[string]any{
			"type":        "field",
			"ruleTag":     xrayrule.DiagnosticsMarker("client", ep.Tag),
			"ip":          []string{markerIP + "/32"},
			"port":        fmt.Sprintf("%d", DiagnosticsMarkerPort),
			"outboundTag": ep.Tag,
		})
	}
	return rules, nil
}
