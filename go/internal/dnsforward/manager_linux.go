//go:build linux

package dnsforward

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

type Manager struct {
	dnsConfig   string
	statePath   string
	installDir  string
	configDir   string
	forwardRole string // "client" or "server"
}

type AddOptions struct {
	Domain      string
	Target      string
	WithForward bool
	Intercept   bool
	Quiet       bool
}

type RemoveOptions struct {
	Domain      string
	All         bool
	WithForward bool
	Intercept   bool
	Quiet       bool
}

type ListEntry struct {
	Domain string
	Server string
	Target string
	Labels []string
}

func NewClientManager(installDir, configDir string) (*Manager, error) {
	return newManager("client", installDir, configDir)
}

func NewServerManager(installDir, configDir string) (*Manager, error) {
	return newManager("server", installDir, configDir)
}

func newManager(role, installDir, configDir string) (*Manager, error) {
	dnsCfg, err := detectDNSConfig()
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(installDir)
	if base == "" {
		base = layout.UnixConfigRoot
	}
	return &Manager{
		dnsConfig:   dnsCfg,
		statePath:   filepath.Join(base, "dns-forward-state.json"),
		installDir:  installDir,
		configDir:   configDir,
		forwardRole: role,
	}, nil
}

func (m *Manager) Add(ctx context.Context, opts AddOptions) (ListEntry, error) {
	if err := ensureOpenWrt(); err != nil {
		return ListEntry{}, err
	}
	domain, err := normalizeDomain(opts.Domain)
	if err != nil {
		return ListEntry{}, err
	}
	rebind := baseDomain(domain)
	targetAddr, targetPort, err := parseTarget(opts.Target)
	if err != nil {
		return ListEntry{}, err
	}

	state, _ := loadState(m.statePath)
	serverIP := targetAddr.String()
	serverPort := targetPort
	labels := []string{"xp2p"}
	stateChanged := false

	if opts.WithForward {
		rule, created, err := m.ensureForward(targetAddr, targetPort, state)
		if err != nil {
			return ListEntry{}, err
		}
		serverIP = rule.ListenAddress
		serverPort = rule.ListenPort
		if created {
			labels = append(labels, "forward:auto")
		} else {
			labels = append(labels, "forward:existing")
		}
		state.record(domain, stateEntry{
			Target:            fmt.Sprintf("%s:%d", targetAddr.String(), targetPort),
			Server:            fmt.Sprintf("%s#%d", serverIP, serverPort),
			ForwardListenPort: rule.ListenPort,
			ForwardTag:        rule.Tag,
			AutoForward:       created,
			RebindDomain:      rebind,
		})
		stateChanged = true
	} else {
		forwards, err := m.listForwards()
		if err != nil {
			return ListEntry{}, err
		}
		rule, found, err := selectForward(forwards, opts.Quiet)
		if err != nil {
			return ListEntry{}, err
		}
		if !found {
			return ListEntry{}, fmt.Errorf("xp2p: no forwards configured; add one or use --with-forward")
		}
		serverIP = rule.ListenAddress
		serverPort = rule.ListenPort
		labels = append(labels, "forward:existing")
		state.record(domain, stateEntry{
			Target:            fmt.Sprintf("%s:%d", targetAddr.String(), targetPort),
			Server:            fmt.Sprintf("%s#%d", serverIP, serverPort),
			ForwardListenPort: rule.ListenPort,
			ForwardTag:        rule.Tag,
			AutoForward:       false,
			RebindDomain:      rebind,
		})
		stateChanged = true
	}

	serverValue := fmt.Sprintf("%s#%d", serverIP, serverPort)
	if err := m.upsertDNSMasq(domain, serverValue); err != nil {
		return ListEntry{}, err
	}
	if err := m.ensureRebindAllowed(rebind); err != nil {
		return ListEntry{}, err
	}

	if opts.Intercept {
		if err := m.ensureIntercept(); err != nil {
			return ListEntry{}, err
		}
		labels = append(labels, "intercept")
	}

	if err := m.commitDNS(); err != nil {
		return ListEntry{}, err
	}
	if err := m.reloadDNS(); err != nil {
		return ListEntry{}, err
	}
	if opts.Intercept {
		if err := m.commitFirewall(); err != nil {
			return ListEntry{}, err
		}
		if err := m.reloadFirewall(); err != nil {
			return ListEntry{}, err
		}
	}

	if stateChanged {
		if err := state.save(m.statePath); err != nil {
			return ListEntry{}, err
		}
	}

	entry := ListEntry{
		Domain: domain,
		Server: serverValue,
		Target: fmt.Sprintf("%s:%d", targetAddr.String(), targetPort),
		Labels: labels,
	}
	return entry, nil
}

func (m *Manager) Remove(opts RemoveOptions) ([]string, error) {
	if err := ensureOpenWrt(); err != nil {
		return nil, err
	}
	state, _ := loadState(m.statePath)
	stateChanged := false

	var domains []string
	if opts.All {
		for domain := range state.Entries {
			domains = append(domains, domain)
		}
	} else {
		domain, err := normalizeDomain(opts.Domain)
		if err != nil {
			return nil, err
		}
		domains = []string{domain}
	}

	if len(domains) == 0 {
		return nil, errors.New("xp2p: no dns-forward entries found")
	}

	removedCount := 0
	dnsSection, err := m.dnsmasqSection()
	if err != nil {
		return nil, err
	}
	for _, domain := range domains {
		rebind := baseDomain(domain)
		entry, hasState := state.Entries[domain]

		// Remove list server entries
		value := fmt.Sprintf("/%s/%s", domain, entry.Server)
		_ = runCommand("uci", "del_list", fmt.Sprintf("%s.%s.server=%s", m.dnsConfig, dnsSection, value))

		// Remove legacy per-domain sections if any
		sections, err := m.readDNSSections()
		if err == nil {
			for name, sec := range sections {
				if !sec.isManagedDNS() {
					continue
				}
				if sec.option("name") != domain {
					continue
				}
				_ = runCommand("uci", "delete", fmt.Sprintf("%s.%s", m.dnsConfig, name))
			}
		}

		if opts.WithForward && hasState && entry.ForwardListenPort > 0 && entry.AutoForward {
			m.removeForward(entry.ForwardListenPort)
		}
		if hasState {
			state.remove(domain)
			stateChanged = true
			removedCount++
		}
		if !m.rebindInUse(rebind, domains, sections, state) {
			_ = m.removeRebind(rebind)
		}
	}
	if removedCount == 0 && !opts.All {
		return nil, fmt.Errorf("xp2p: dns-forward entry for %s not found", domains[0])
	}

	if err := m.commitDNS(); err != nil {
		return nil, err
	}
	if err := m.reloadDNS(); err != nil {
		return nil, err
	}

	if opts.Intercept {
		_ = m.removeIntercept()
		if err := m.commitFirewall(); err == nil {
			_ = m.reloadFirewall()
		}
	}

	if stateChanged {
		if err := state.save(m.statePath); err != nil {
			return nil, err
		}
	}

	sort.Strings(domains)
	return domains, nil
}

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
		return forward.Rule{}, false, fmt.Errorf("xp2p: no forwards configured; add one or use --with-forward")
	}
	if len(forwards) == 1 {
		return forwards[0], true, nil
	}
	if quiet {
		return forward.Rule{}, false, fmt.Errorf("xp2p: multiple forwards found; rerun with --with-forward to create one automatically")
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
