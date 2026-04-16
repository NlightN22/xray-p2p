package extensions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	RoutingAfterSystemFile  = "routing.rules.after-xp2p-system.json"
	RoutingAfterManagedFile = "routing.rules.after-xp2p-managed.json"
	InboundsAppendFile      = "inbounds.append.json"
	OutboundsAppendFile     = "outbounds.append.json"
)

type Snippets struct {
	RoutingAfterSystem  []any
	RoutingAfterManaged []any
	InboundsAppend      []any
	OutboundsAppend     []any
}

func Load(dir string) (Snippets, error) {
	clean := filepath.Clean(strings.TrimSpace(dir))
	if clean == "" {
		return Snippets{}, fmt.Errorf("extensions: dir is empty")
	}
	afterSystem, err := readArrayObject(clean, RoutingAfterSystemFile, "rules")
	if err != nil {
		return Snippets{}, err
	}
	afterManaged, err := readArrayObject(clean, RoutingAfterManagedFile, "rules")
	if err != nil {
		return Snippets{}, err
	}
	inbounds, err := readArrayObject(clean, InboundsAppendFile, "inbounds")
	if err != nil {
		return Snippets{}, err
	}
	outbounds, err := readArrayObject(clean, OutboundsAppendFile, "outbounds")
	if err != nil {
		return Snippets{}, err
	}
	return Snippets{
		RoutingAfterSystem:  afterSystem,
		RoutingAfterManaged: afterManaged,
		InboundsAppend:      inbounds,
		OutboundsAppend:     outbounds,
	}, nil
}

func readArrayObject(dir, name, key string) ([]any, error) {
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("extensions: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("extensions: parse %s: %w", path, err)
	}
	raw, ok := doc[key]
	if !ok {
		return nil, fmt.Errorf("extensions: %s missing %q", path, key)
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("extensions: %s has invalid %q", path, key)
	}
	return arr, nil
}
