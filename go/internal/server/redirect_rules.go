//go:build linux || windows

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

const serverRedirectRulesKey = "server_redirects"

// RedirectAddOptions controls server redirect creation.
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

// RedirectRemoveOptions controls server redirect deletion.
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

// RedirectListOptions controls redirect enumeration.
type RedirectListOptions struct {
	InstallDir string
	ConfigDir  string
	Pending    bool
}

// RedirectRecord describes a server redirect.
type RedirectRecord struct {
	Type     string
	Value    string
	CIDR     string
	Domain   string
	Tag      string
	Hostname string
}

type serverRedirectStore struct {
	path      string
	doc       map[string]any
	reverse   serverReverseState
	redirects []redirect.Rule
}

func openServerRedirectStore(installDir string) (serverRedirectStore, error) {
	path := serverStatePath(installDir)
	doc, err := loadServerStateDoc(path)
	if err != nil {
		return serverRedirectStore{}, err
	}
	return buildServerRedirectStore(path, doc)
}

func openServerRedirectStoreFromPath(path string) (serverRedirectStore, error) {
	doc, err := loadServerStateDoc(path)
	if err != nil {
		return serverRedirectStore{}, err
	}
	return buildServerRedirectStore(path, doc)
}

func buildServerRedirectStore(path string, doc map[string]any) (serverRedirectStore, error) {
	reverseState, err := decodeServerReverseState(doc)
	if err != nil {
		return serverRedirectStore{}, err
	}
	redirects, err := decodeServerRedirectRules(doc)
	if err != nil {
		return serverRedirectStore{}, err
	}
	return serverRedirectStore{
		path:      path,
		doc:       doc,
		reverse:   reverseState,
		redirects: redirects,
	}, nil
}

func decodeServerRedirectRules(doc map[string]any) ([]redirect.Rule, error) {
	raw := doc[serverRedirectRulesKey]
	if raw == nil {
		return []redirect.Rule{}, nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode server redirect state: %w", err)
	}
	var rules []redirect.Rule
	if err := json.Unmarshal(buf, &rules); err != nil {
		return nil, fmt.Errorf("parse server redirect state: %w", err)
	}
	return rules, nil
}

func (s *serverRedirectStore) saveRedirects() error {
	if s.doc == nil {
		s.doc = make(map[string]any)
	}
	if len(s.redirects) == 0 {
		s.doc[serverRedirectRulesKey] = nil
	} else {
		s.doc[serverRedirectRulesKey] = s.redirects
	}
	return writeServerStateDoc(s.path, s.doc)
}

func (s serverRedirectStore) bindings() []redirect.Binding {
	if len(s.reverse) == 0 {
		return nil
	}
	result := make([]redirect.Binding, 0, len(s.reverse))
	for _, channel := range s.reverse {
		result = append(result, redirect.Binding{
			Tag:  channel.Tag,
			Host: channel.Host,
		})
	}
	return result
}

// AddRedirect registers a server-side redirect for reverse portals.
func AddRedirect(opts RedirectAddOptions) error {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	store, err := openServerRedirectStorePending()
	if err != nil {
		return err
	}
	if len(store.reverse) == 0 {
		return errors.New("no reverse portals configured (add xp2p server users first)")
	}

	binding, err := resolveServerRedirectBinding(opts.Tag, opts.Hostname, store.bindings())
	if err != nil {
		return err
	}

	target, err := redirect.ResolveRule(opts.CIDR, opts.Domain)
	if err != nil {
		return err
	}

	rule := redirect.Rule{
		OutboundTag: binding.Tag,
	}
	if target.Kind == redirect.KindDomain {
		rule.Domain = target.Value
	} else {
		rule.CIDR = target.Value
		rule.NoRoutes = opts.NoRoutes
	}

	updated, addErr := redirect.AddRule(store.redirects, rule)
	if addErr != nil && !errors.Is(addErr, redirect.ErrRuleExists) {
		return addErr
	}
	if errors.Is(addErr, redirect.ErrRuleExists) {
		return nil
	}
	store.redirects = updated
	if err := store.saveRedirects(); err != nil {
		return err
	}
	_ = installDir
	return writeServerApplyRequest()
}

// RemoveRedirect removes a server redirect rule.
func RemoveRedirect(opts RedirectRemoveOptions) error {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	store, err := openServerRedirectStorePending()
	if err != nil {
		return err
	}
	if len(store.redirects) == 0 {
		return errors.New("no server redirect rules configured")
	}

	target, err := redirect.ResolveRule(opts.CIDR, opts.Domain)
	if err != nil {
		return err
	}

	tagFilter := strings.TrimSpace(opts.Tag)
	if strings.TrimSpace(opts.Hostname) != "" {
		binding, bindErr := resolveServerRedirectBinding(tagFilter, opts.Hostname, store.bindings())
		if bindErr != nil {
			return bindErr
		}
		tagFilter = binding.Tag
	}

	updated, removed := redirect.RemoveRule(store.redirects, target, tagFilter)
	if !removed {
		return fmt.Errorf("redirect %s not found", target.Describe())
	}
	store.redirects = updated
	if err := store.saveRedirects(); err != nil {
		return err
	}
	_ = installDir
	return writeServerApplyRequest()
}

// ListRedirects lists configured server redirects.
func ListRedirects(opts RedirectListOptions) ([]RedirectRecord, error) {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return nil, err
	}

	var store serverRedirectStore
	if opts.Pending {
		store, err = openServerRedirectStoreFromPath(pendingConfigPath())
	} else {
		store, err = openServerRedirectStore(installDir)
	}
	if err != nil {
		return nil, err
	}

	tagToHost := make(map[string]string, len(store.reverse))
	for _, channel := range store.reverse {
		tagToHost[strings.ToLower(channel.Tag)] = channel.Host
	}

	records := make([]RedirectRecord, 0, len(store.redirects))
	for _, rule := range store.redirects {
		recType := "CIDR"
		val := rule.Value()
		if rule.Kind() == redirect.KindDomain {
			recType = "domain"
		}
		host := tagToHost[strings.ToLower(rule.OutboundTag)]
		records = append(records, RedirectRecord{
			Type:     recType,
			Value:    val,
			CIDR:     rule.CIDR,
			Domain:   rule.Domain,
			Tag:      rule.OutboundTag,
			Hostname: host,
		})
	}
	return records, nil
}

func resolveServerRedirectBinding(tag, host string, bindings []redirect.Binding) (redirect.Binding, error) {
	trimmedTag := strings.TrimSpace(tag)
	trimmedHost := strings.TrimSpace(host)
	if trimmedTag != "" {
		var matched redirect.Binding
		found := false
		for _, binding := range bindings {
			if strings.EqualFold(binding.Tag, trimmedTag) {
				matched = binding
				found = true
				break
			}
		}
		if !found {
			return redirect.Binding{}, fmt.Errorf("outbound tag %q is not registered", trimmedTag)
		}
		if trimmedHost != "" && !strings.EqualFold(strings.TrimSpace(matched.Host), trimmedHost) {
			resolvedHost := matched.Host
			if strings.TrimSpace(resolvedHost) == "" {
				resolvedHost = trimmedHost
			}
			return redirect.Binding{}, fmt.Errorf("tag %q does not match reverse host %q", trimmedTag, resolvedHost)
		}
		return matched, nil
	}

	binding, err := redirect.ResolveBinding(tag, host, bindings)
	if err != nil {
		switch {
		case errors.Is(err, redirect.ErrBindingNotSpecified):
			return redirect.Binding{}, errors.New("--tag or --host is required")
		case errors.Is(err, redirect.ErrBindingHostNotFound):
			return redirect.Binding{}, fmt.Errorf("reverse portal host %q not found", strings.TrimSpace(host))
		case errors.Is(err, redirect.ErrBindingTagNotFound):
			return redirect.Binding{}, fmt.Errorf("outbound tag %q is not registered", strings.TrimSpace(tag))
		case errors.Is(err, redirect.ErrBindingTagMismatch):
			resolvedHost := binding.Host
			if strings.TrimSpace(resolvedHost) == "" {
				resolvedHost = strings.TrimSpace(host)
			}
			return redirect.Binding{}, fmt.Errorf("tag %q does not match reverse host %q", tag, resolvedHost)
		default:
			return redirect.Binding{}, err
		}
	}
	return binding, nil
}
