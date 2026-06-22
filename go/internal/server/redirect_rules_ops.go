//go:build linux || windows

package server

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

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

	binding, err := resolveServerRedirectChannel(opts.Tag, opts.User, opts.Hostname, store.reverse)
	if err != nil {
		return err
	}

	target, err := redirect.ResolveRule(opts.CIDR, opts.Domain)
	if err != nil {
		return err
	}

	rule := redirect.Rule{
		OutboundTag:  binding.Tag,
		AccessPolicy: redirect.AccessPolicy{Access: opts.Access, Users: opts.AllowUsers, Groups: opts.AllowGroups},
	}
	if rule.AccessPolicy, err = rule.AccessPolicy.Normalized(); err != nil {
		return err
	}
	if target.Kind == redirect.KindDomain {
		rule.Domain = target.Value
	} else {
		rule.CIDR = target.Value
		rule.NoRoutes = opts.NoRoutes
	}

	current := store.redirects
	if strings.TrimSpace(opts.Hostname) != "" {
		current, _ = redirect.RemoveRule(current, target, "")
	}
	updated, addErr := redirect.AddRule(current, rule)
	if addErr != nil && !errors.Is(addErr, redirect.ErrRuleExists) {
		return addErr
	}
	if errors.Is(addErr, redirect.ErrRuleExists) {
		return nil
	}
	previous := append([]redirect.Rule(nil), store.redirects...)
	store.redirects = updated
	store.doc[serverRedirectRulesKey] = store.redirects
	_ = installDir
	return commitServerRedirectRuntimeDoc(context.Background(), store.doc, previous, store.redirects)
}

func SetRedirectAccess(opts RedirectAccessOptions) error {
	store, err := openServerRedirectStorePending()
	if err != nil {
		return err
	}
	target, err := redirect.ResolveRule(opts.CIDR, opts.Domain)
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.Tag) == "" {
		return errors.New("--tag is required")
	}
	if strings.TrimSpace(opts.Hostname) != "" {
		binding, e := resolveServerRedirectBinding(opts.Tag, opts.Hostname, store.bindings())
		if e != nil {
			return e
		}
		opts.Tag = binding.Tag
	}
	policy, err := (redirect.AccessPolicy{Access: opts.Access, Users: opts.AllowUsers, Groups: opts.AllowGroups}).Normalized()
	if err != nil {
		return err
	}
	found := false
	for i := range store.redirects {
		if target.Matches(store.redirects[i]) && strings.EqualFold(store.redirects[i].OutboundTag, opts.Tag) {
			store.redirects[i].AccessPolicy = policy
			found = true
		}
	}
	if !found {
		return fmt.Errorf("redirect %s via %s not found", target.Describe(), opts.Tag)
	}
	previous := append([]redirect.Rule(nil), store.redirects...)
	store.doc[serverRedirectRulesKey] = store.redirects
	return commitServerRedirectRuntimeDoc(context.Background(), store.doc, previous, store.redirects)
}

// UpdateRedirectAccess applies one selector mutation while preserving the rest of the policy.
func UpdateRedirectAccess(opts RedirectAccessOptions, action string) error {
	store, err := openServerRedirectStorePending()
	if err != nil {
		return err
	}
	target, err := redirect.ResolveRule(opts.CIDR, opts.Domain)
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.Tag) == "" {
		return errors.New("--tag is required")
	}
	found := -1
	for i := range store.redirects {
		if target.Matches(store.redirects[i]) && strings.EqualFold(store.redirects[i].OutboundTag, opts.Tag) {
			found = i
			break
		}
	}
	if found < 0 {
		return fmt.Errorf("redirect %s via %s not found", target.Describe(), opts.Tag)
	}
	p := store.redirects[found].AccessPolicy
	switch action {
	case "clear":
		p = redirect.AccessPolicy{Access: "all"}
	case "add-user":
		p.Users = append(p.Users, opts.AllowUsers...)
	case "remove-user":
		p.Users = removeAccessValues(p.Users, opts.AllowUsers)
	case "add-group":
		p.Groups = append(p.Groups, opts.AllowGroups...)
	case "remove-group":
		p.Groups = removeAccessValues(p.Groups, opts.AllowGroups)
	default:
		return fmt.Errorf("unknown redirect access action %q", action)
	}
	if action != "clear" && p.Access == "" {
		p.Access = "restricted"
	}
	p, err = p.Normalized()
	if err != nil {
		return err
	}
	previous := append([]redirect.Rule(nil), store.redirects...)
	store.redirects[found].AccessPolicy = p
	store.doc[serverRedirectRulesKey] = store.redirects
	return commitServerRedirectRuntimeDoc(context.Background(), store.doc, previous, store.redirects)
}

func removeAccessValues(values, remove []string) []string {
	removed := make(map[string]struct{}, len(remove))
	for _, value := range remove {
		removed[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := removed[strings.ToLower(strings.TrimSpace(value))]; !ok {
			out = append(out, value)
		}
	}
	return out
}

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

	tagFilter := strings.TrimSpace(opts.Tag)
	hasTarget := strings.TrimSpace(opts.CIDR) != "" || strings.TrimSpace(opts.Domain) != ""
	if !hasTarget && tagFilter == "" {
		return errors.New("--cidr, --domain, or --tag is required")
	}
	if strings.TrimSpace(opts.Hostname) != "" {
		binding, bindErr := resolveServerRedirectBinding(tagFilter, opts.Hostname, store.bindings())
		if bindErr != nil {
			return bindErr
		}
		tagFilter = binding.Tag
	}

	var (
		updated []redirect.Rule
		removed bool
	)
	if hasTarget {
		target, err := redirect.ResolveRule(opts.CIDR, opts.Domain)
		if err != nil {
			return err
		}
		updated, removed = redirect.RemoveRule(store.redirects, target, tagFilter)
		if !removed {
			return fmt.Errorf("redirect %s not found", target.Describe())
		}
	} else {
		updated, removed = redirect.RemoveRulesByTag(store.redirects, tagFilter)
		if !removed {
			return fmt.Errorf("redirect tag %s not found", tagFilter)
		}
	}
	previous := append([]redirect.Rule(nil), store.redirects...)
	store.redirects = updated
	store.doc[serverRedirectRulesKey] = store.redirects
	_ = installDir
	return commitServerRedirectRuntimeDoc(context.Background(), store.doc, previous, store.redirects)
}

func SetRedirectEnabled(opts RedirectSetEnabledOptions) error {
	store, err := openServerRedirectStorePending()
	if err != nil {
		return err
	}
	if len(store.redirects) == 0 {
		return errors.New("no server redirect rules configured")
	}

	target := redirect.Target{}
	if !opts.All {
		target, err = redirect.ResolveRule(opts.CIDR, opts.Domain)
		if err != nil {
			return err
		}
	}
	tagFilter := strings.TrimSpace(opts.Tag)
	if strings.TrimSpace(opts.Hostname) != "" {
		binding, bindErr := resolveServerRedirectBinding(tagFilter, opts.Hostname, store.bindings())
		if bindErr != nil {
			return bindErr
		}
		tagFilter = binding.Tag
	}
	updated, changed := redirect.SetRulesEnabled(store.redirects, target, tagFilter, opts.All, opts.Enabled)
	if !changed {
		if opts.All {
			return nil
		}
		return fmt.Errorf("redirect %s not found", target.Describe())
	}
	previous := append([]redirect.Rule(nil), store.redirects...)
	store.redirects = updated
	store.doc[serverRedirectRulesKey] = store.redirects
	return commitServerRedirectRuntimeDoc(context.Background(), store.doc, previous, store.redirects)
}

func commitServerRedirectRuntimeDoc(ctx context.Context, doc map[string]any, previous []redirect.Rule, current []redirect.Rule) error {
	result, err := commitServerRuntimeDocResult(ctx, doc)
	if err != nil {
		return err
	}
	if result == xraylive.RuntimeApplyStaged {
		return nil
	}
	return syncServerRedirectRoutesAfterRuntimeApply(previous, current)
}

func syncServerRedirectRoutesAfterRuntimeApply(previous []redirect.Rule, current []redirect.Rule) error {
	cfg, err := loadServerConfigWithFallback()
	if err != nil {
		return err
	}
	if !cfg.Server.TunEnabled {
		return nil
	}
	if err := removeRedirectRoutes(cfg.Server.TunName, cfg.Server.TunAddr, previous); err != nil {
		return err
	}
	if err := applyRedirectRoutes(cfg.Server.TunName, cfg.Server.TunAddr, current); err != nil {
		return err
	}
	desired, err := loadServerDesiredConfigFromPath(pendingConfigPath())
	if err != nil {
		return err
	}
	appliedStatePath := filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName))
	if err := saveServerAppliedState(appliedStatePath, desired.Reverse, desired.Redirects, desired.Forwards, cfg.Server.TunEnabled, cfg.Server.TunName, cfg.Server.TunMTU, cfg.Server.TunAddr); err != nil {
		return err
	}
	logging.Info("server redirect routes reconciled", "count", len(current))
	return nil
}

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
			Type:             recType,
			Value:            val,
			CIDR:             rule.CIDR,
			Domain:           rule.Domain,
			Tag:              rule.OutboundTag,
			Hostname:         host,
			Disabled:         rule.Disabled,
			DisabledByPolicy: redirectDisabledByPolicy(rule),
		})
	}
	return records, nil
}

func redirectDisabledByPolicy(rule redirect.Rule) bool {
	policy, err := rule.AccessPolicy.Normalized()
	return err == nil && policy.Access == "restricted" && len(policy.Users) == 0
}
