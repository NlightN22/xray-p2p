package client

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
)

// RedirectAddOptions controls redirect creation.
type RedirectAddOptions struct {
	InstallDir string
	ConfigDir  string
	CIDR       string
	Domain     string
	Tag        string
	Hostname   string
}

// RedirectRemoveOptions controls redirect removal.
type RedirectRemoveOptions struct {
	InstallDir string
	ConfigDir  string
	CIDR       string
	Domain     string
	Tag        string
	Hostname   string
}

// RedirectListOptions configures listing.
type RedirectListOptions struct {
	InstallDir string
	ConfigDir  string
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

type redirectPaths struct {
	clientPaths
	routing string
}

// AddRedirect registers a custom CIDR redirect.
func AddRedirect(opts RedirectAddOptions) error {
	paths, err := resolveRedirectPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	state, err := loadClientInstallState(paths.stateFile)
	if err != nil {
		return err
	}
	if len(state.Endpoints) == 0 {
		return errors.New("xp2p: no client endpoints found (run xp2p client install first)")
	}

	tag, _, err := resolveRedirectTarget(opts.Tag, opts.Hostname, state.Endpoints)
	if err != nil {
		return err
	}

	ruleTarget, err := resolveRedirectRule(opts.CIDR, opts.Domain)
	if err != nil {
		return err
	}

	rule := clientRedirectRule{
		OutboundTag: tag,
	}
	switch ruleTarget.kind {
	case redirectRuleTypeDomain:
		rule.Domain = ruleTarget.value
	default:
		rule.CIDR = ruleTarget.value
	}
	if err := state.addRedirect(rule); err != nil {
		return err
	}
	if err := state.save(paths.stateFile); err != nil {
		return err
	}
	return updateRoutingConfig(paths.routing, state.Endpoints, state.Redirects)
}

// RemoveRedirect deletes redirect rules.
func RemoveRedirect(opts RedirectRemoveOptions) error {
	paths, err := resolveRedirectPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	state, err := loadClientInstallState(paths.stateFile)
	if err != nil {
		return err
	}
	if len(state.Redirects) == 0 {
		return errors.New("xp2p: no redirect rules configured")
	}

	ruleTarget, err := resolveRedirectRule(opts.CIDR, opts.Domain)
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
		return fmt.Errorf("xp2p: redirect %s not found", ruleTarget.describe())
	}
	state.Redirects = updated
	if err := state.save(paths.stateFile); err != nil {
		return err
	}
	return updateRoutingConfig(paths.routing, state.Endpoints, state.Redirects)
}

// ListRedirects returns configured redirect entries.
func ListRedirects(opts RedirectListOptions) ([]RedirectRecord, error) {
	paths, err := resolveRedirectPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return nil, err
	}

	state, err := loadClientInstallState(paths.stateFile)
	if err != nil {
		return nil, err
	}

	tagToHost := make(map[string]string, len(state.Endpoints))
	for _, ep := range state.Endpoints {
		tagToHost[strings.ToLower(ep.Tag)] = ep.Hostname
	}

	records := make([]RedirectRecord, 0, len(state.Redirects))
	for _, rule := range state.Redirects {
		host := tagToHost[strings.ToLower(rule.OutboundTag)]
		recType := rule.kind().label()
		value := rule.value()
		records = append(records, RedirectRecord{
			Type:     recType,
			Value:    value,
			CIDR:     rule.CIDR,
			Domain:   rule.Domain,
			Tag:      rule.OutboundTag,
			Hostname: host,
		})
	}
	return records, nil
}

func resolveRedirectPaths(installDir, configDir string) (redirectPaths, error) {
	paths, err := resolveClientPaths(installDir, configDir)
	if err != nil {
		return redirectPaths{}, err
	}
	return redirectPaths{
		clientPaths: paths,
		routing:     filepath.Join(paths.configDir, "routing.json"),
	}, nil
}

func resolveRedirectTarget(tag, host string, endpoints []clientEndpointRecord) (string, string, error) {
	trimmedTag := strings.TrimSpace(tag)
	trimmedHost := strings.TrimSpace(host)

	if trimmedTag == "" && trimmedHost == "" {
		return "", "", errors.New("xp2p: --tag or --host is required")
	}

	var matched clientEndpointRecord
	if trimmedHost != "" {
		found := false
		for _, ep := range endpoints {
			if strings.EqualFold(ep.Hostname, trimmedHost) {
				matched = ep
				found = true
				break
			}
		}
		if !found {
			return "", "", fmt.Errorf("xp2p: client endpoint %q not found", trimmedHost)
		}
		trimmedTag = matched.Tag
	} else {
		found := false
		for _, ep := range endpoints {
			if strings.EqualFold(ep.Tag, trimmedTag) {
				matched = ep
				found = true
				break
			}
		}
		if !found {
			return "", "", fmt.Errorf("xp2p: outbound tag %q is not registered", trimmedTag)
		}
	}

	if strings.TrimSpace(tag) != "" && !strings.EqualFold(tag, matched.Tag) {
		return "", "", fmt.Errorf("xp2p: tag %q does not match host %q", tag, matched.Hostname)
	}

	return matched.Tag, matched.Hostname, nil
}

func resolveRedirectRule(cidr, domain string) (redirectTarget, error) {
	hasCIDR := strings.TrimSpace(cidr) != ""
	hasDomain := strings.TrimSpace(domain) != ""
	switch {
	case hasCIDR && hasDomain:
		return redirectTarget{}, errors.New("xp2p: specify only one of --cidr or --domain")
	case !hasCIDR && !hasDomain:
		return redirectTarget{}, errors.New("xp2p: --cidr or --domain is required")
	case hasCIDR:
		normalized, err := normalizeCIDR(cidr)
		if err != nil {
			return redirectTarget{}, err
		}
		return redirectTarget{kind: redirectRuleTypeCIDR, value: normalized}, nil
	default:
		normalized, err := normalizeDomain(domain)
		if err != nil {
			return redirectTarget{}, err
		}
		return redirectTarget{kind: redirectRuleTypeDomain, value: normalized}, nil
	}
}

func normalizeCIDR(value string) (string, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return "", errors.New("xp2p: --cidr is required")
	}
	_, network, err := net.ParseCIDR(clean)
	if err != nil {
		return "", fmt.Errorf("xp2p: invalid CIDR %q: %w", value, err)
	}
	return network.String(), nil
}

func normalizeDomain(value string) (string, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return "", errors.New("xp2p: --domain is required")
	}
	if strings.ContainsAny(clean, " \t\r\n") {
		return "", fmt.Errorf("xp2p: invalid domain %q", value)
	}
	return strings.ToLower(clean), nil
}
