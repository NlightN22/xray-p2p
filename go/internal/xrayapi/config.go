package xrayapi

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIListenFromConfig reads api.listen from a compiled Xray config.
func APIListenFromConfig(data []byte) (string, error) {
	var doc struct {
		API struct {
			Listen string `json:"listen"`
		} `json:"api"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("parse xray config: %w", err)
	}
	return strings.TrimSpace(doc.API.Listen), nil
}
