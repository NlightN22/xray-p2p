package client

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

const (
	socksHealthTimeout  = 30 * time.Second
	socksHealthInterval = 500 * time.Millisecond
)

func resolveClientSocksAddress(xrayJSONPath string) (string, error) {
	defaults := xrayconfig.DefaultClientConfig().Inbounds.Socks
	data, err := os.ReadFile(xrayJSONPath)
	if err != nil {
		return socksAddress(defaults.Listen, defaults.Port, defaults.Listen, defaults.Port), err
	}
	listen, port, parseErr := findSocksInbound(data)
	if parseErr != nil {
		return socksAddress(defaults.Listen, defaults.Port, defaults.Listen, defaults.Port), parseErr
	}
	return socksAddress(listen, port, defaults.Listen, defaults.Port), nil
}

func socksAddress(listen string, port int, fallbackListen string, fallbackPort int) string {
	host := strings.TrimSpace(listen)
	if host == "" {
		host = strings.TrimSpace(fallbackListen)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = fallbackPort
	}
	if port <= 0 {
		port = 51180
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func findSocksInbound(data []byte) (string, int, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		return "", 0, err
	}
	raw, ok := root["inbounds"]
	if !ok {
		return "", 0, nil
	}
	entries, ok := raw.([]any)
	if !ok {
		return "", 0, nil
	}
	for _, entryRaw := range entries {
		entry, ok := entryRaw.(map[string]any)
		if !ok {
			continue
		}
		proto, _ := entry["protocol"].(string)
		if !strings.EqualFold(strings.TrimSpace(proto), "socks") {
			continue
		}
		listen, _ := entry["listen"].(string)
		port := 0
		switch v := entry["port"].(type) {
		case json.Number:
			if value, err := v.Int64(); err == nil {
				port = int(value)
			}
		case float64:
			port = int(v)
		}
		return listen, port, nil
	}
	return "", 0, nil
}
