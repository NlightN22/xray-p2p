// Command configschema generates editor schemas from the persisted Go models.
// Generated files must not be edited manually.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	output := flag.String("output", "schemas", "output directory")
	check := flag.Bool("check", false, "check generated files without changing them")
	flag.Parse()

	items := []struct {
		name, id, title, description string
		model                        any
	}{
		{"xp2p-client.schema.json", "https://github.com/NlightN22/xray-p2p/schemas/xp2p-client.schema.json", "xp2p client TOML configuration", "Schema for xp2p-client.toml Desired inputs.", clientRoot{}},
		{"xp2p-server.schema.json", "https://github.com/NlightN22/xray-p2p/schemas/xp2p-server.schema.json", "xp2p server TOML configuration", "Schema for xp2p-server.toml Desired inputs.", serverRoot{}},
	}
	if err := os.MkdirAll(*output, 0o755); err != nil {
		fatal(err)
	}
	for _, item := range items {
		r := newReflector()
		doc := r.root(item.model, item.id, item.title, item.description)
		applyOverlays(doc)
		data, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			fatal(err)
		}
		data = append(bytes.TrimSpace(data), '\n')
		path := filepath.Join(*output, item.name)
		if *check {
			current, err := os.ReadFile(path)
			if err != nil {
				fatal(err)
			}
			if !bytes.Equal(current, data) {
				fatal(fmt.Errorf("schema drift detected in %s; run make schema", path))
			}
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fatal(err)
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
