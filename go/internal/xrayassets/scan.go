package xrayassets

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

var (
	geoIPRef    = regexp.MustCompile(`(?i)^geoip:`)
	geositeRef  = regexp.MustCompile(`(?i)^geosite:`)
	extAssetRef = regexp.MustCompile(`(?i)^ext:([^:]+):`)
)

func RequiredFromXrayConfig(path string) ([]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read xray config %s: %w", path, err)
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse xray config %s: %w", path, err)
	}
	found := map[string]struct{}{}
	walk(doc, found)
	out := make([]string, 0, len(found))
	for name := range found {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func walk(value any, found map[string]struct{}) {
	switch v := value.(type) {
	case map[string]any:
		for _, item := range v {
			walk(item, found)
		}
	case []any:
		for _, item := range v {
			walk(item, found)
		}
	case string:
		scanString(v, found)
	}
}

func scanString(value string, found map[string]struct{}) {
	for _, part := range strings.Fields(value) {
		part = strings.TrimSpace(part)
		if geoIPRef.MatchString(part) {
			found["geoip.dat"] = struct{}{}
		}
		if geositeRef.MatchString(part) {
			found["geosite.dat"] = struct{}{}
		}
		if match := extAssetRef.FindStringSubmatch(part); len(match) == 2 {
			found[strings.TrimSpace(match[1])] = struct{}{}
		}
	}
}
