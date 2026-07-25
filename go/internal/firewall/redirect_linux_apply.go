//go:build linux

package firewall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func (m Manager) ApplyPlan(plan Plan) (Plan, error) {
	if err := m.persistEntries(plan); err != nil {
		return plan, err
	}
	if plan.Snippet != "" {
		if err := os.MkdirAll(filepath.Dir(plan.SnippetPath), 0o755); err != nil {
			return plan, fmt.Errorf("nat redirect: create snippet dir: %w", err)
		}
		if err := os.WriteFile(plan.SnippetPath, []byte(plan.Snippet), 0o644); err != nil {
			return plan, fmt.Errorf("nat redirect: write snippet: %w", err)
		}
	} else {
		_ = os.Remove(plan.SnippetPath)
	}
	if plan.Backend == "fw4" {
		if !commandExists("fw4") {
			return plan, fmt.Errorf("nat redirect: fw4 binary not found")
		}
		cmd := exec.Command("fw4", "reload")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return plan, fmt.Errorf("nat redirect: fw4 reload failed: %w. output: %s", err, string(output))
		}
		if plan.Snippet == "" {
			if err := removeFW4RuntimeChains(); err != nil {
				return plan, err
			}
		}
		return plan, nil
	}
	if plan.Backend == "nft" {
		if !commandExists("nft") {
			return plan, fmt.Errorf("nat redirect: nft binary not found")
		}
		if plan.Snippet != "" {
			if err := exec.Command("nft", "-f", plan.SnippetPath).Run(); err != nil {
				return plan, fmt.Errorf("nat redirect: nft apply: %w", err)
			}
		} else {
			_ = exec.Command("nft", "delete", "table", "inet", "xray_transparent").Run()
		}
		return plan, nil
	}
	plan.Backend = "iptables"
	if err := applyIPTables(plan); err != nil {
		return plan, err
	}
	return plan, nil
}

func removeFW4RuntimeChains() error {
	for _, chain := range []string{
		"xray_transparent_prerouting",
		"xray_transparent_output",
		"xray_transparent_output_allow",
	} {
		if err := exec.Command("nft", "list", "chain", "inet", "fw4", chain).Run(); err != nil {
			continue
		}
		output, err := exec.Command("nft", "delete", "chain", "inet", "fw4", chain).CombinedOutput()
		if err != nil {
			return fmt.Errorf("nat redirect: remove fw4 chain %s: %w. output: %s",
				chain, err, string(output))
		}
	}
	return nil
}

func (m Manager) persistEntries(plan Plan) error {
	if err := os.MkdirAll(m.entryDir, 0o755); err != nil {
		return fmt.Errorf("nat redirect: create entry dir: %w", err)
	}
	if plan.RemoveAll {
		matches, _ := filepath.Glob(filepath.Join(m.entryDir, "*.entry"))
		for _, path := range matches {
			_ = os.Remove(path)
		}
	}
	if plan.EntryPath == "" {
		return nil
	}
	if plan.Snippet == "" {
		return os.Remove(plan.EntryPath)
	}
	if plan.Entry == nil {
		return os.Remove(plan.EntryPath)
	}
	if plan.Entry.Port <= 0 || plan.Entry.CIDR == "" {
		return fmt.Errorf("nat redirect: missing entry details")
	}
	content := fmt.Sprintf("CIDR=\"%s\"\nPORT=\"%d\"\n", plan.Entry.CIDR, plan.Entry.Port)
	if err := os.WriteFile(plan.EntryPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("nat redirect: write entry: %w", err)
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("nat redirect: empty entry")
	}
	return nil
}

func entryPathForCIDR(dir, cidr string) string {
	clean := strings.ToLower(cidr)
	clean = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, clean)
	return filepath.Join(dir, fmt.Sprintf("xray_redirect_%s.entry", clean))
}

func readEntry(path string) (Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, err
	}
	lines := strings.Split(string(data), "\n")
	entry := Entry{}
	for _, line := range lines {
		if strings.HasPrefix(line, "CIDR=") {
			entry.CIDR = strings.Trim(strings.TrimPrefix(line, "CIDR="), "\"")
		}
		if strings.HasPrefix(line, "PORT=") {
			val := strings.Trim(strings.TrimPrefix(line, "PORT="), "\"")
			entry.Port, _ = strconv.Atoi(val)
		}
	}
	return entry, nil
}

func upsertEntry(entries []Entry, add Entry) []Entry {
	var updated []Entry
	found := false
	for _, e := range entries {
		if e.CIDR == add.CIDR {
			updated = append(updated, add)
			found = true
			continue
		}
		updated = append(updated, e)
	}
	if !found {
		updated = append(updated, add)
	}
	return updated
}
