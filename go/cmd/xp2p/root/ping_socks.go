package root

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func resolveSocksAddress(cfg config.Config, flagValue string) (string, error) {
	value := strings.TrimSpace(flagValue)
	switch value {
	case "":
		return "", nil
	case tunnelConfigSentinel:
		return detectSocksProxy(cfg)
	}

	return normalizeSocksAddress(value)
}

func normalizeSocksAddress(value string) (string, error) {
	host, port, err := splitHostPort(value)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(host, port), nil
}

var errSocksInboundNotFound = errors.New("socks inbound not found")

func detectSocksProxy(cfg config.Config) (string, error) {
	if addr, err := loadSocksAddress(cfg.Client.InstallDir, cfg.Client.ConfigDir); err == nil {
		return addr, nil
	} else if !errors.Is(err, errSocksInboundNotFound) {
		return "", err
	}

	if addr, err := loadSocksAddress(cfg.Server.InstallDir, cfg.Server.ConfigDir); err == nil {
		return addr, nil
	} else if !errors.Is(err, errSocksInboundNotFound) {
		return "", err
	}

	if cfg.Client.SocksAddress != "" {
		if addr, err := normalizeSocksAddress(cfg.Client.SocksAddress); err == nil {
			return addr, nil
		}
	}

	return "", fmt.Errorf("tunnel proxy not configured; specify --tunnel host:port or install xp2p client/server")
}

func detectSocksProxies(cfg config.Config) (string, string, error) {
	clientAddr, err := loadSocksAddress(cfg.Client.InstallDir, cfg.Client.ConfigDir)
	if err != nil && !errors.Is(err, errSocksInboundNotFound) {
		return "", "", err
	}
	if err != nil && errors.Is(err, errSocksInboundNotFound) {
		clientAddr = ""
	}

	serverAddr, err := loadSocksAddress(cfg.Server.InstallDir, cfg.Server.ConfigDir)
	if err != nil && !errors.Is(err, errSocksInboundNotFound) {
		return "", "", err
	}
	if err != nil && errors.Is(err, errSocksInboundNotFound) {
		serverAddr = ""
	}

	if clientAddr == "" && cfg.Client.SocksAddress != "" && clientConfigPresent() {
		if addr, err := normalizeSocksAddress(cfg.Client.SocksAddress); err == nil {
			clientAddr = addr
		}
	}

	return clientAddr, serverAddr, nil
}

func clientConfigPresent() bool {
	live := config.LiveConfigPath(layout.ClientConfigFileName)
	_, err := os.Stat(live)
	if err == nil {
		return true
	}
	return false
}

func resolveAutoSocks(preferServer bool, clientAddr, serverAddr string) (string, error) {
	if preferServer {
		if serverAddr != "" {
			return serverAddr, nil
		}
		return "", fmt.Errorf("server tunnel proxy not configured; specify --tunnel host:port or install xp2p server")
	}
	if clientAddr != "" {
		return clientAddr, nil
	}
	if serverAddr != "" {
		return serverAddr, nil
	}
	return "", fmt.Errorf("tunnel proxy not configured; specify --tunnel host:port or install xp2p client/server")
}

func loadSocksAddress(installDir, configDir string) (string, error) {
	dir := strings.TrimSpace(configDir)
	if dir == "" {
		return "", errSocksInboundNotFound
	}
	if !filepath.IsAbs(dir) {
		base := strings.TrimSpace(installDir)
		if base == "" {
			return "", errSocksInboundNotFound
		}
		dir = filepath.Join(base, dir)
	}

	liveDir, err := config.LiveConfigDir(dir)
	if err != nil || strings.TrimSpace(liveDir) == "" {
		return "", errSocksInboundNotFound
	}
	path := filepath.Join(liveDir, layout.XrayConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errSocksInboundNotFound
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}

	raw, ok := root["inbounds"]
	if !ok {
		return "", fmt.Errorf("%s missing \"inbounds\" array", path)
	}
	entries, ok := raw.([]any)
	if !ok {
		return "", fmt.Errorf("%s has invalid \"inbounds\" array", path)
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

		host := ""
		if listenRaw, ok := entry["listen"]; ok {
			value, err := stringifyListen(listenRaw)
			if err != nil {
				return "", fmt.Errorf("%s: %w", path, err)
			}
			host = value
		}
		if host == "" {
			host = "127.0.0.1"
		}

		port, err := parseInboundPort(entry["port"])
		if err != nil {
			return "", fmt.Errorf("%s: %w", path, err)
		}
		return net.JoinHostPort(host, port), nil
	}

	return "", errSocksInboundNotFound
}
