//go:build linux || windows

package client

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

// RedirectAddOptions controls redirect creation.
type RedirectAddOptions struct {
	InstallDir string
	ConfigDir  string
	CIDR       string
	Domain     string
	Tag        string
	Hostname   string
	NoRoutes   bool
	TunEnabled bool
	TunName    string
}

// RedirectRemoveOptions controls redirect removal.
type RedirectRemoveOptions struct {
	InstallDir string
	ConfigDir  string
	CIDR       string
	Domain     string
	Tag        string
	Hostname   string
	TunEnabled bool
	TunName    string
}

// RedirectListOptions configures listing.
type RedirectListOptions struct {
	InstallDir string
	ConfigDir  string
	Pending    bool
}

// RedirectRecord describes a redirect rule.
type RedirectRecord struct {
	Type     string
	Value    string
	CIDR     string
	Domain   string
	Tag      string
	Hostname string
}

// AddRedirect registers a custom CIDR redirect.
func AddRedirect(opts RedirectAddOptions) error {
	configFile := config.ConfigPath(layout.ClientConfigFileName)
	state, err := loadClientInstallState(configFile)
	if err != nil {
		return err
	}
	if len(state.Endpoints) == 0 {
		return errors.New("no client endpoints found (run xp2p client install first)")
	}

	tag, _, err := resolveRedirectTarget(opts.Tag, opts.Hostname, state.Endpoints)
	if err != nil {
		return err
	}

	ruleTarget, err := redirect.ResolveRule(opts.CIDR, opts.Domain)
	if err != nil {
		return err
	}
	if ruleTarget.Kind == redirect.KindCIDR && isDefaultRoute(ruleTarget.Value) {
		return errors.New("default route redirects are reserved for tun-mode full")
	}
	rule := redirect.Rule{
		OutboundTag: tag,
	}
	switch ruleTarget.Kind {
	case redirect.KindDomain:
		rule.Domain = ruleTarget.Value
	default:
		rule.CIDR = ruleTarget.Value
		rule.NoRoutes = opts.NoRoutes
	}
	addErr := state.addRedirect(rule)
	if addErr != nil && !errors.Is(addErr, redirect.ErrRuleExists) {
		return addErr
	}
	if errors.Is(addErr, redirect.ErrRuleExists) {
		return nil
	}
	if err := state.save(configFile); err != nil {
		return err
	}
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
}

func isDefaultRoute(value string) bool {
	ip, network, err := net.ParseCIDR(strings.TrimSpace(value))
	if err != nil || ip == nil || network == nil {
		return false
	}
	ones, _ := network.Mask.Size()
	if ones != 0 {
		return false
	}
	if ip.To4() != nil {
		return ip.Equal(net.IPv4zero)
	}
	return ip.Equal(net.IPv6zero)
}

// RemoveRedirect deletes redirect rules.
func RemoveRedirect(opts RedirectRemoveOptions) error {
	configFile := config.ConfigPath(layout.ClientConfigFileName)
	state, err := loadClientInstallState(configFile)
	if err != nil {
		return err
	}
	if len(state.Redirects) == 0 {
		return errors.New("no redirect rules configured")
	}

	ruleTarget, err := redirect.ResolveRule(opts.CIDR, opts.Domain)
	if err != nil {
		return err
	}

	tagFilter := strings.TrimSpace(opts.Tag)
	if strings.TrimSpace(opts.Hostname) != "" {
		var resolved string
		resolved, _, err = resolveRedirectTarget(tagFilter, opts.Hostname, state.Endpoints)
		if err != nil {
			return err
		}
		tagFilter = resolved
	}

	updated, removed := state.removeRedirect(ruleTarget, tagFilter)
	if !removed {
		return fmt.Errorf("redirect %s not found", ruleTarget.Describe())
	}
	state.Redirects = updated
	if err := state.save(configFile); err != nil {
		return err
	}
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
}

// ListRedirects returns configured redirect entries.
func ListRedirects(opts RedirectListOptions) ([]RedirectRecord, error) {
	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	state, err := loadClientInstallState(statePath)
	if err != nil {
		return nil, err
	}
	_ = opts.Pending
	return buildRedirectRecords(state), nil
}

func buildRedirectRecords(state clientInstallState) []RedirectRecord {
	tagToHost := make(map[string]string, len(state.Endpoints))
	for _, ep := range state.Endpoints {
		tagToHost[strings.ToLower(ep.Tag)] = ep.Hostname
	}

	records := make([]RedirectRecord, 0, len(state.Redirects)+len(state.Endpoints))
	seen := make(map[string]struct{}, len(state.Redirects)+len(state.Endpoints))

	addRecord := func(rec RedirectRecord) {
		if rec.Value == "" || rec.Tag == "" {
			return
		}
		key := strings.ToLower(rec.Type) + "|" + strings.ToLower(rec.Value) + "|" + strings.ToLower(rec.Tag)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		records = append(records, rec)
	}

	for _, rule := range state.Redirects {
		host := tagToHost[strings.ToLower(rule.OutboundTag)]
		recType := "CIDR"
		if rule.Kind() == redirect.KindDomain {
			recType = "domain"
		}
		value := rule.Value()
		addRecord(RedirectRecord{
			Type:     recType,
			Value:    value,
			CIDR:     rule.CIDR,
			Domain:   rule.Domain,
			Tag:      rule.OutboundTag,
			Hostname: host,
		})
	}
	return records
}

func resolveRedirectTarget(tag, host string, endpoints []clientEndpointRecord) (string, string, error) {
	bindings := make([]redirect.Binding, 0, len(endpoints))
	for _, ep := range endpoints {
		bindings = append(bindings, redirect.Binding{
			Tag:  ep.Tag,
			Host: ep.Hostname,
		})
	}
	binding, err := redirect.ResolveBinding(tag, host, bindings)
	if err != nil {
		switch {
		case errors.Is(err, redirect.ErrBindingNotSpecified):
			return "", "", errors.New("--tag or --host is required")
		case errors.Is(err, redirect.ErrBindingHostNotFound):
			return "", "", fmt.Errorf("client endpoint %q not found", strings.TrimSpace(host))
		case errors.Is(err, redirect.ErrBindingTagNotFound):
			return "", "", fmt.Errorf("outbound tag %q is not registered", strings.TrimSpace(tag))
		case errors.Is(err, redirect.ErrBindingTagMismatch):
			resolvedHost := binding.Host
			if strings.TrimSpace(resolvedHost) == "" {
				resolvedHost = strings.TrimSpace(host)
			}
			return "", "", fmt.Errorf("tag %q does not match host %q", tag, resolvedHost)
		default:
			return "", "", err
		}
	}
	return binding.Tag, binding.Host, nil
}
