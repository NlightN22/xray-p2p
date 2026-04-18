//go:build linux

package dnsforward

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
)

func (m *Manager) List() ([]ListEntry, bool, error) {
	if err := ensureOpenWrt(); err != nil {
		return nil, false, err
	}
	state, _ := loadState(m.statePath)

	intercept := m.interceptPresent()
	var entries []ListEntry
	for domain, s := range state.Entries {
		entry := ListEntry{
			Domain: domain,
			Server: s.Server,
			Target: s.Target,
			Labels: []string{"xp2p"},
		}
		if s.ForwardListenPort > 0 {
			if s.AutoForward {
				entry.Labels = append(entry.Labels, "forward:auto")
			} else {
				entry.Labels = append(entry.Labels, "forward:recorded")
			}
		}
		if intercept {
			entry.Labels = append(entry.Labels, "intercept")
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Domain < entries[j].Domain })
	return entries, intercept, nil
}

func selectForward(forwards []forward.Rule, quiet bool) (forward.Rule, bool, error) {
	if len(forwards) == 0 {
		return forward.Rule{}, false, fmt.Errorf("no forwards configured; add one or use --with-forward")
	}
	if len(forwards) == 1 {
		return forwards[0], true, nil
	}
	if quiet {
		return forward.Rule{}, false, fmt.Errorf("multiple forwards found; rerun with --with-forward to create one automatically")
	}

	fmt.Println("Select a forward to use for DNS:")
	for i, fwd := range forwards {
		fmt.Printf("%d) %s:%d -> %s (%s)\n", i+1, fwd.ListenAddress, fwd.ListenPort, fwd.Target(), fwd.NetworkValue())
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter number: ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		val, err := strconv.Atoi(line)
		if err != nil || val < 1 || val > len(forwards) {
			fmt.Println("Invalid selection.")
			continue
		}
		return forwards[val-1], true, nil
	}
}
