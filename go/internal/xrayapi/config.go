package xrayapi

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// APIListenFromConfig reads the API listen address from a compiled Xray config.
func APIListenFromConfig(data []byte) (string, error) {
	var doc struct {
		API struct {
			Tag    string `json:"tag"`
			Listen string `json:"listen"`
		} `json:"api"`
		Inbounds []struct {
			Tag    string `json:"tag"`
			Listen string `json:"listen"`
			Port   int    `json:"port"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("parse xray config: %w", err)
	}
	if listen := strings.TrimSpace(doc.API.Listen); listen != "" {
		return listen, nil
	}
	apiTag := strings.TrimSpace(doc.API.Tag)
	if apiTag == "" {
		apiTag = "api"
	}
	for _, inbound := range doc.Inbounds {
		if strings.TrimSpace(inbound.Tag) != apiTag || inbound.Port <= 0 {
			continue
		}
		host := strings.TrimSpace(inbound.Listen)
		if host == "" {
			host = "127.0.0.1"
		}
		return net.JoinHostPort(host, fmt.Sprintf("%d", inbound.Port)), nil
	}
	return "", nil
}
