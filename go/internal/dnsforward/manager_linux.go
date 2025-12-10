//go:build linux

package dnsforward

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

type Manager struct {
	dnsConfig  string
	statePath  string
	installDir string
	configDir  string
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

func NewManager(installDir, configDir string) (*Manager, error) {
	dnsCfg, err := detectDNSConfig()
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(installDir)
	if base == "" {
		base = layout.UnixConfigRoot
	}
	return &Manager{
		dnsConfig:  dnsCfg,
		statePath:  filepath.Join(base, "dns-forward-state.json"),
		installDir: installDir,
		configDir:  configDir,
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

	if opts.WithForward {
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

	sections, err := m.readDNSSections()
	if err != nil {
		return nil, err
	}

	var domains []string
	if opts.All {
		for _, sec := range sections {
			if sec.isManagedDNS() {
				if name := sec.option("name"); name != "" {
					domains = append(domains, name)
				}
			}
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
	for _, domain := range domains {
		rebind := baseDomain(domain)
		for name, sec := range sections {
			if !sec.isManagedDNS() {
				continue
			}
			if sec.option("name") != domain {
				continue
			}
			if err := runCommand("uci", "delete", fmt.Sprintf("%s.%s", m.dnsConfig, name)); err != nil {
				return nil, err
			}
			removedCount++
		}

		if opts.WithForward {
			if entry, ok := state.Entries[domain]; ok && entry.ForwardListenPort > 0 && entry.AutoForward {
				_, _ = client.RemoveForward(client.ForwardRemoveOptions{
					InstallDir: m.installDir,
					ConfigDir:  m.configDir,
					Selector: forward.Selector{
						ListenPort: entry.ForwardListenPort,
					},
				})
			}
			state.remove(domain)
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

	if opts.WithForward {
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

	sections, err := m.readDNSSections()
	if err != nil {
		return nil, false, err
	}

	intercept := m.interceptPresent()
	var entries []ListEntry
	for _, sec := range sections {
		if !sec.isManagedDNS() {
			continue
		}
		domain := sec.option("name")
		server := sec.option("server")
		if domain == "" || server == "" {
			continue
		}
		entry := ListEntry{
			Domain: domain,
			Server: server,
			Target: server,
			Labels: []string{"xp2p"},
		}
		if s, ok := state.Entries[domain]; ok && s.ForwardListenPort > 0 {
			if s.AutoForward {
				entry.Labels = append(entry.Labels, "forward:auto")
			} else {
				entry.Labels = append(entry.Labels, "forward:recorded")
			}
			entry.Target = s.Target
		}
		if intercept {
			entry.Labels = append(entry.Labels, "intercept")
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Domain < entries[j].Domain })
	return entries, intercept, nil
}
