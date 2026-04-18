package clientcmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func listClientModeCompletions(cfg config.Config, cmd *cobra.Command, wantTags bool) []string {
	installDir, _ := cmd.Flags().GetString("path")
	configDir, _ := cmd.Flags().GetString("config-dir")
	installDir = firstNonEmpty(strings.TrimSpace(installDir), cfg.Client.InstallDir)
	configDir = firstNonEmpty(strings.TrimSpace(configDir), cfg.Client.ConfigDir)
	bindings, err := listClientBindings(installDir, configDir)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(bindings))
	seen := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		value := strings.TrimSpace(binding.Host)
		if wantTags {
			value = strings.TrimSpace(binding.Tag)
		}
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func filterCompletions(values []string, prefix string) []string {
	trimmed := strings.ToLower(strings.TrimSpace(prefix))
	if trimmed == "" {
		return values
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(strings.ToLower(value), trimmed) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
