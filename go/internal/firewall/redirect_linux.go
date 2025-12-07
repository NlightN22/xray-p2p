//go:build linux

package firewall

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Entry struct {
	Subnet string
	Port   int
}

type Plan struct {
	Snippet     string
	SnippetPath string
	EntryPath   string
	IPTables    []string
	Backend     string
	RemoveAll   bool
	Entry       *Entry
	UseFW4      bool
}

type Manager struct {
	snippetPath string
	entryDir    string
	useFW4      bool
}

func fw4Available() bool {
	for _, candidate := range []string{"/usr/sbin/fw4", "/sbin/fw4", "/usr/bin/fw4", "/bin/fw4"} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return true
		}
	}
	return commandExists("fw4")
}

func NewManager(snippetPath, entryDir string) Manager {
	useFW4 := fw4Available()
	return Manager{
		snippetPath: strings.TrimSpace(snippetPath),
		entryDir:    strings.TrimSpace(entryDir),
		useFW4:      useFW4 && strings.TrimSpace(snippetPath) != "",
	}
}

func (m Manager) List() ([]Entry, error) {
	paths, err := filepath.Glob(filepath.Join(m.entryDir, "*.entry"))
	if err != nil {
		return nil, fmt.Errorf("nat redirect: list entries: %w", err)
	}
	sort.Strings(paths)
	var entries []Entry
	for _, p := range paths {
		e, err := readEntry(p)
		if err == nil && e.Subnet != "" && e.Port > 0 {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (m Manager) PlanAdd(subnet string, port int) (Plan, error) {
	if err := validateCIDR(subnet); err != nil {
		return Plan{}, err
	}
	if port <= 0 || port > 65535 {
		return Plan{}, fmt.Errorf("nat redirect: port must be between 1 and 65535")
	}
	entries, err := m.List()
	if err != nil {
		return Plan{}, err
	}
	updated := upsertEntry(entries, Entry{Subnet: subnet, Port: port})
	snippet := renderSnippetWithFW4(updated, m.useFW4)
	entryPath := entryPathForSubnet(m.entryDir, subnet)
	backend := selectBackend()
	if m.useFW4 {
		backend = "fw4"
	}
	return Plan{
		Snippet:     snippet,
		SnippetPath: m.snippetPath,
		EntryPath:   entryPath,
		IPTables:    renderIPTables(updated),
		Backend:     backend,
		Entry:       &Entry{Subnet: subnet, Port: port},
		UseFW4:      m.useFW4,
	}, nil
}

func (m Manager) PlanRemove(subnet string, all bool) (Plan, error) {
	entries, err := m.List()
	if err != nil {
		return Plan{}, err
	}
	var updated []Entry
	var entryPath string
	switch {
	case all:
		updated = nil
		entryPath = ""
	case subnet != "":
		if err := validateCIDR(subnet); err != nil {
			return Plan{}, err
		}
		entryPath = entryPathForSubnet(m.entryDir, subnet)
		for _, e := range entries {
			if e.Subnet != subnet {
				updated = append(updated, e)
			}
		}
	default:
		return Plan{}, fmt.Errorf("nat redirect: subnet or --all required")
	}
	snippet := renderSnippetWithFW4(updated, m.useFW4)
	backend := selectBackend()
	if m.useFW4 {
		backend = "fw4"
	}
	return Plan{
		Snippet:     snippet,
		SnippetPath: m.snippetPath,
		EntryPath:   entryPath,
		IPTables:    renderIPTables(updated),
		Backend:     backend,
		RemoveAll:   all,
		UseFW4:      m.useFW4,
	}, nil
}
