package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
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
}

// RedirectRemoveOptions controls server redirect deletion.
type RedirectRemoveOptions struct {
	InstallDir string
	ConfigDir  string
	CIDR       string
	Domain     string
	Tag        string
	Hostname   string
}

// RedirectListOptions controls redirect enumeration.
type RedirectListOptions struct {
	InstallDir string
	ConfigDir  string
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
		return nil, fmt.Errorf("xp2p: encode server redirect state: %w", err)
	}
	var rules []redirect.Rule
	if err := json.Unmarshal(buf, &rules); err != nil {
		return nil, fmt.Errorf("xp2p: parse server redirect state: %w", err)
	}
	return rules, nil
}

func (s *serverRedirectStore) saveRedirects() error {
	if s.doc == nil {
		s.doc = make(map[string]any)
	}
	if len(s.redirects) == 0 {
		delete(s.doc, serverRedirectRulesKey)
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

	configDir, err := resolveUserConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	store, err := openServerRedirectStore(installDir)
	if err != nil {
		return err
	}
	if len(store.reverse) == 0 {
		return errors.New("xp2p: no reverse portals configured (add xp2p server users first)")
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
	}

	updated, err := redirect.AddRule(store.redirects, rule)
	if err != nil {
		return err
	}
	store.redirects = updated
	if err := store.saveRedirects(); err != nil {
		return err
	}

	routingPath := filepath.Join(configDir, "routing.json")
	return updateServerRedirectRouting(routingPath, store.redirects)
}

// RemoveRedirect removes a server redirect rule.
func RemoveRedirect(opts RedirectRemoveOptions) error {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	configDir, err := resolveUserConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	store, err := openServerRedirectStore(installDir)
	if err != nil {
		return err
	}
	if len(store.redirects) == 0 {
		return errors.New("xp2p: no server redirect rules configured")
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
		return fmt.Errorf("xp2p: redirect %s not found", target.Describe())
	}
	store.redirects = updated
	if err := store.saveRedirects(); err != nil {
		return err
	}

	routingPath := filepath.Join(configDir, "routing.json")
	return updateServerRedirectRouting(routingPath, store.redirects)
}

// ListRedirects lists configured server redirects.
func ListRedirects(opts RedirectListOptions) ([]RedirectRecord, error) {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return nil, err
	}

	store, err := openServerRedirectStore(installDir)
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
	binding, err := redirect.ResolveBinding(tag, host, bindings)
	if err != nil {
		switch {
		case errors.Is(err, redirect.ErrBindingNotSpecified):
			return redirect.Binding{}, errors.New("xp2p: --tag or --host is required")
		case errors.Is(err, redirect.ErrBindingHostNotFound):
			return redirect.Binding{}, fmt.Errorf("xp2p: reverse portal host %q not found", strings.TrimSpace(host))
		case errors.Is(err, redirect.ErrBindingTagNotFound):
			return redirect.Binding{}, fmt.Errorf("xp2p: outbound tag %q is not registered", strings.TrimSpace(tag))
		case errors.Is(err, redirect.ErrBindingTagMismatch):
			resolvedHost := binding.Host
			if strings.TrimSpace(resolvedHost) == "" {
				resolvedHost = strings.TrimSpace(host)
			}
			return redirect.Binding{}, fmt.Errorf("xp2p: tag %q does not match reverse host %q", tag, resolvedHost)
		default:
			return redirect.Binding{}, err
		}
	}
	return binding, nil
}

func updateServerRedirectRouting(path string, rules []redirect.Rule) error {
	doc, err := loadServerRouting(path)
	if err != nil {
		return err
	}
	routing := ensureObject(doc, "routing")
	existing := extractInterfaceSlice(routing["rules"])
	filtered := filterServerRedirectRules(existing)
	for _, rule := range rules {
		filtered = append(filtered, buildServerRedirectRule(rule))
	}
	routing["rules"] = filtered
	return writeServerRouting(path, doc)
}

func filterServerRedirectRules(rules []any) []any {
	if len(rules) == 0 {
		return []any{}
	}
	filtered := make([]any, 0, len(rules))
	for _, raw := range rules {
		ruleMap, ok := raw.(map[string]any)
		if !ok || !isServerRedirectRule(ruleMap) {
			filtered = append(filtered, raw)
			continue
		}
	}
	return filtered
}

func isServerRedirectRule(ruleMap map[string]any) bool {
	if typ, _ := ruleMap["type"].(string); !strings.EqualFold(strings.TrimSpace(typ), "field") {
		return false
	}
	if inbound := extractStringSlice(ruleMap["inboundTag"]); len(inbound) > 0 {
		return false
	}
	outbound, _ := ruleMap["outboundTag"].(string)
	if strings.TrimSpace(outbound) == "" {
		return false
	}
	hasDomains := len(extractStringSlice(ruleMap["domains"])) > 0
	hasIP := len(extractStringSlice(ruleMap["ip"])) > 0
	if hasDomains && hasIP {
		return false
	}
	if !hasDomains && !hasIP {
		return false
	}
	if len(extractStringSlice(ruleMap["domain"])) > 0 {
		return false
	}
	if len(extractStringSlice(ruleMap["user"])) > 0 {
		return false
	}
	return true
}

func buildServerRedirectRule(rule redirect.Rule) map[string]any {
	entry := map[string]any{
		"type":        "field",
		"outboundTag": rule.OutboundTag,
	}
	if rule.Kind() == redirect.KindDomain {
		entry["domains"] = []string{rule.Value()}
	} else {
		entry["ip"] = []string{rule.Value()}
	}
	return entry
}
