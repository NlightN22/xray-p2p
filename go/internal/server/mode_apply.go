//go:build linux || windows

package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func applyServerMode(installDir, configDir string, opts ModeOptions) error {
	desired, err := loadServerDesiredConfig(installDir)
	if err != nil {
		return err
	}
	applied, err := loadServerAppliedState(filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName)))
	if err != nil {
		return err
	}
	if err := applyServerDesiredConfig(installDir, configDir, desired, applied.Reverse, opts); err != nil {
		return err
	}
	return saveServerAppliedState(filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName)), desired.Reverse, desired.Redirects, desired.Forwards, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr)
}

func applyServerDesiredConfig(installDir, configDir string, desired desiredServerConfig, previousReverse serverReverseState, opts ModeOptions) error {
	previousReverse = normalizeReverse(previousReverse)
	if err := updateServerInbounds(configDir, opts.TunEnabled, opts.TunName, opts.TunMTU); err != nil {
		return err
	}
	if err := syncServerForwardInbounds(configDir, desired.Forwards); err != nil {
		return err
	}
	for tag, channel := range previousReverse {
		if _, ok := desired.Reverse[tag]; ok {
			continue
		}
		if err := removeServerRoutingConfig(configDir, channel); err != nil {
			return err
		}
	}
	for _, channel := range desired.Reverse {
		if err := ensureServerRoutingConfig(configDir, channel); err != nil {
			return err
		}
	}
	if err := ensureServerMarkerRules(configDir, desired.Reverse); err != nil {
		return err
	}
	if err := updateServerRedirectRouting(filepath.Join(configDir, "routing.json"), desired.Redirects); err != nil {
		return err
	}
	if opts.TunEnabled {
		return applyRedirectRoutes(opts.TunName, desired.Redirects)
	}
	return removeRedirectRoutes(opts.TunName, desired.Redirects)
}

func updateServerInbounds(configDir string, tunEnabled bool, tunName string, tunMTU int) error {
	path := filepath.Join(configDir, "inbounds.json")
	doc, ok, err := readJSONDoc(path)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("xp2p: inbounds.json not found at %s", path)
	}
	updated := updateTunInbounds(extractInterfaces(doc["inbounds"]), tunEnabled, tunName, tunMTU)
	doc["inbounds"] = updated
	return writeJSONDoc(path, doc)
}

func readJSONDoc(path string) (map[string]any, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("xp2p: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, false, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, false, fmt.Errorf("xp2p: parse %s: %w", path, err)
	}
	return doc, true, nil
}

func writeJSONDoc(path string, doc map[string]any) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("xp2p: encode %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write %s: %w", path, err)
	}
	return nil
}

func extractInterfaces(raw any) []any {
	if values, ok := raw.([]any); ok {
		return values
	}
	return []any{}
}

func updateTunInbounds(inbounds []any, tunEnabled bool, tunName string, tunMTU int) []any {
	var updated []any
	var tunInbound map[string]any
	for _, raw := range inbounds {
		entry, ok := raw.(map[string]any)
		if !ok {
			updated = append(updated, raw)
			continue
		}
		proto, _ := entry["protocol"].(string)
		if strings.EqualFold(strings.TrimSpace(proto), "tun") {
			if tunEnabled && tunInbound == nil {
				tunInbound = entry
			}
			continue
		}
		updated = append(updated, entry)
	}
	if tunEnabled {
		if tunInbound == nil {
			tunInbound = newTunInbound(tunName, tunMTU)
		} else {
			updateTunInboundSettings(tunInbound, tunName, tunMTU)
		}
		updated = append([]any{tunInbound}, updated...)
	}
	return updated
}

func newTunInbound(tunName string, tunMTU int) map[string]any {
	return map[string]any{
		"tag":      "tun-in",
		"port":     0,
		"protocol": "tun",
		"settings": map[string]any{
			"name": strings.TrimSpace(tunName),
			"mtu":  tunMTU,
		},
	}
}

func updateTunInboundSettings(inbound map[string]any, tunName string, tunMTU int) {
	settings, ok := inbound["settings"].(map[string]any)
	if !ok {
		settings = make(map[string]any)
		inbound["settings"] = settings
	}
	settings["name"] = strings.TrimSpace(tunName)
	settings["mtu"] = tunMTU
	if _, ok := inbound["tag"].(string); !ok {
		inbound["tag"] = "tun-in"
	}
	if _, ok := inbound["port"].(float64); !ok {
		inbound["port"] = 0
	}
	if _, ok := inbound["protocol"].(string); !ok {
		inbound["protocol"] = "tun"
	}
}
