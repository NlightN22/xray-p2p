//go:build linux

package dnsforward

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type uciSection struct {
	name    string
	kind    string
	options map[string][]string
}

func (s uciSection) option(name string) string {
	values := s.options[name]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (s uciSection) isManagedDNS() bool {
	if s.kind != "server" && s.kind != "domain" {
		return false
	}
	for _, val := range s.options["xp2p"] {
		if strings.TrimSpace(val) == "1" {
			return true
		}
	}
	return false
}

func parseUCIShow(output string) map[string]uciSection {
	result := make(map[string]uciSection)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		chunks := strings.Split(key, ".")
		if len(chunks) < 2 {
			continue
		}
		sectionName := chunks[1]
		section := result[sectionName]
		section.name = sectionName
		if len(chunks) == 2 {
			section.kind = val
		} else {
			if section.options == nil {
				section.options = make(map[string][]string)
			}
			optionName := chunks[2]
			section.options[optionName] = parseUCIValues(val)
		}
		result[sectionName] = section
	}
	return result
}

func parseUCIValues(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := strings.Fields(raw)
	values := make([]string, 0, len(fields))
	for _, f := range fields {
		values = append(values, strings.Trim(f, "\"'"))
	}
	return values
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("xp2p: %s %s: %v (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(buf.String()))
	}
	return nil
}

func captureCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}
