package clientcmd

import (
	"errors"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/cli/tagprompt"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func resolveFullTunnelBinding(installDir, configDir, tag, host, existingTag string, quiet bool) (string, string, error) {
	tagValue := strings.TrimSpace(tag)
	hostValue := strings.TrimSpace(host)
	if tagValue == "" && hostValue == "" {
		if strings.TrimSpace(existingTag) != "" {
			return strings.TrimSpace(existingTag), "", nil
		}
		if quiet {
			return "", "", errors.New("--tag or --host is required for full-tunnel")
		}
		selection, err := promptClientRedirectBinding(installDir, configDir)
		if err != nil {
			if errors.Is(err, tagprompt.ErrEmpty) || errors.Is(err, tagprompt.ErrAborted) {
				return "", "", errors.New("--tag or --host is required for full-tunnel")
			}
			return "", "", err
		}
		return selection.Tag, selection.Host, nil
	}

	bindings, err := listClientBindings(installDir, configDir)
	if err != nil {
		return "", "", err
	}
	binding, err := redirect.ResolveBinding(tagValue, hostValue, bindings)
	if err != nil {
		switch {
		case errors.Is(err, redirect.ErrBindingHostNotFound):
			return "", "", errors.New("client endpoint not found")
		case errors.Is(err, redirect.ErrBindingTagNotFound):
			return "", "", errors.New("outbound tag is not registered")
		case errors.Is(err, redirect.ErrBindingTagMismatch):
			return binding.Tag, binding.Host, errors.New("tag does not match host")
		default:
			return "", "", err
		}
	}
	return binding.Tag, binding.Host, nil
}

func listClientBindings(installDir, configDir string) ([]redirect.Binding, error) {
	records, err := clientListFunc(client.ListOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		Pending:    true,
	})
	if err != nil {
		return nil, err
	}
	bindings := make([]redirect.Binding, 0, len(records))
	for _, rec := range records {
		if strings.TrimSpace(rec.Tag) == "" {
			continue
		}
		bindings = append(bindings, redirect.Binding{
			Tag:  rec.Tag,
			Host: rec.Hostname,
		})
	}
	return bindings, nil
}
