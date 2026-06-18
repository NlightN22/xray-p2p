//go:build linux || windows

package server

import (
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
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
	store.redirects = updated
	if err := store.saveRedirects(); err != nil {
		return err
	}
	return writeServerApplyRequest()
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
			Type:     recType,
			Value:    val,
			CIDR:     rule.CIDR,
			Domain:   rule.Domain,
			Tag:      rule.OutboundTag,
			Hostname: host,
			Disabled: rule.Disabled,
		})
	}
	return records, nil
}
