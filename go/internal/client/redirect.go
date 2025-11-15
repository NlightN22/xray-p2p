package client

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

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

	ruleTarget, err := redirect.ResolveRule(opts.CIDR, opts.Domain)
	if err != nil {
		return err
	}

	rule := redirect.Rule{
		OutboundTag: tag,
	}
	switch ruleTarget.Kind {
	case redirect.KindDomain:
		rule.Domain = ruleTarget.Value
	default:
		rule.CIDR = ruleTarget.Value
	}
	if err := state.addRedirect(rule); err != nil {
		return err
	}
	if err := state.save(paths.stateFile); err != nil {
		return err
	}
	return updateRoutingConfig(paths.routing, state.Endpoints, state.Redirects, state.Reverse)
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
		return fmt.Errorf("xp2p: redirect %s not found", ruleTarget.Describe())
	}
	state.Redirects = updated
	if err := state.save(paths.stateFile); err != nil {
		return err
	}
	return updateRoutingConfig(paths.routing, state.Endpoints, state.Redirects, state.Reverse)
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
		recType := "CIDR"
		if rule.Kind() == redirect.KindDomain {
			recType = "domain"
		}
		value := rule.Value()
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
			return "", "", errors.New("xp2p: --tag or --host is required")
		case errors.Is(err, redirect.ErrBindingHostNotFound):
			return "", "", fmt.Errorf("xp2p: client endpoint %q not found", strings.TrimSpace(host))
		case errors.Is(err, redirect.ErrBindingTagNotFound):
			return "", "", fmt.Errorf("xp2p: outbound tag %q is not registered", strings.TrimSpace(tag))
		case errors.Is(err, redirect.ErrBindingTagMismatch):
			resolvedHost := binding.Host
			if strings.TrimSpace(resolvedHost) == "" {
				resolvedHost = strings.TrimSpace(host)
			}
			return "", "", fmt.Errorf("xp2p: tag %q does not match host %q", tag, resolvedHost)
		default:
			return "", "", err
		}
	}
	return binding.Tag, binding.Host, nil
}
