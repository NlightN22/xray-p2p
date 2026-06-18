package clientcmd

import (
	"errors"
	"strings"

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
		selection, err := resolveClientBinding(clientBindingRequest{
			InstallDir: installDir,
			ConfigDir:  configDir,
			Header:     "Available client endpoints:",
			Reader:     clientRedirectPromptReader(),
			Quiet:      quiet,
		})
		if err != nil {
			if clientBindingRequiredError(err) {
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
	return listClientEndpointBindings(installDir, configDir)
}
