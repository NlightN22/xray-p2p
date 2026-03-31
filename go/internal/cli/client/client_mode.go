package clientcmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/cli/tagprompt"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func newClientModeCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode [tun|proxy] [split|full]",
		Short: "Switch client mode between TUN and proxy (optional tun mode)",
		Args:  cobra.RangeArgs(0, 2),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return []string{"tun", "proxy"}, cobra.ShellCompDirectiveNoFileComp
			case 1:
				if strings.ToLower(strings.TrimSpace(args[0])) == "tun" {
					return []string{"split", "full"}, cobra.ShellCompDirectiveNoFileComp
				}
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			if configFlag := cmd.InheritedFlags().Lookup("config"); configFlag != nil && configFlag.Changed {
				forwarded = append(forwarded, "--config", configFlag.Value.String())
			}
			code := runClientMode(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.StringP("tag", "g", "", "outbound tag for full-tunnel routing (prompts when omitted)")
	flags.StringP("host", "H", "", "client endpoint hostname for full-tunnel routing")
	flags.BoolP("quiet", "q", false, "do not prompt for outbound tags")
	flags.BoolP("verbose", "V", false, "emit full-tunnel change details")
	_ = cmd.RegisterFlagCompletionFunc("tag", func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		tags := listClientModeCompletions(cfg(), cmd, true)
		return filterCompletions(tags, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("host", func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		hosts := listClientModeCompletions(cfg(), cmd, false)
		return filterCompletions(hosts, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func runClientMode(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client mode", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	configPath := fs.String("config", "", "path to configuration file")
	tag := fs.String("tag", "", "outbound tag for full-tunnel routing")
	host := fs.String("host", "", "client endpoint hostname for full-tunnel routing")
	quiet := fs.Bool("quiet", false, "do not prompt for outbound tags")
	verbose := fs.Bool("verbose", false, "emit full-tunnel change details")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client mode: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() == 0 {
		mode, err := resolveClientMode(cfg)
		if err != nil {
			logging.Error("xp2p client mode: failed to resolve current mode", "err", err)
			return 1
		}
		tunMode, err := resolveClientTunMode(*configPath, cfg)
		if err != nil {
			logging.Error("xp2p client mode: failed to resolve tun mode", "err", err)
			return 1
		}
		if mode == "tun" {
			logging.Info("xp2p client mode: current mode", "mode", mode, "tun_mode", tunMode)
		} else {
			logging.Info("xp2p client mode: current mode", "mode", mode)
		}
		return 0
	}
	if fs.NArg() > 2 {
		logging.Error("xp2p client mode: specify tun or proxy (optional split or full)")
		return 2
	}

	mode := strings.ToLower(strings.TrimSpace(fs.Arg(0)))
	tunEnabled, err := parseMode(mode)
	if err != nil {
		logging.Error("xp2p client mode: invalid mode", "err", err)
		return 2
	}

	installDir := firstNonEmpty(*path, cfg.Client.InstallDir)
	configDirName := firstNonEmpty(*configDir, cfg.Client.ConfigDir)
	if strings.TrimSpace(installDir) == "" {
		logging.Error("xp2p client mode: install directory is required")
		return 2
	}

	loadedCfg, err := loadModeConfig(*configPath, cfg)
	if err != nil {
		logging.Error("xp2p client mode: failed to load config", "err", err)
		return 1
	}

	tunMode := loadedCfg.Client.TunMode
	if fs.NArg() == 2 {
		if !tunEnabled {
			logging.Error("xp2p client mode: tun mode is only valid with tun")
			return 2
		}
		value := strings.ToLower(strings.TrimSpace(fs.Arg(1)))
		if value != "split" && value != "full" {
			logging.Error("xp2p client mode: invalid tun mode (use split or full)")
			return 2
		}
		tunMode = value
	}

	fullTunnelTag := strings.TrimSpace(loadedCfg.Client.FullTunnelTag)
	if tunEnabled && tunMode == "full" {
		resolvedTag, _, err := resolveFullTunnelBinding(installDir, configDirName, *tag, *host, fullTunnelTag, *quiet)
		if err != nil {
			logging.Error("xp2p client mode: failed to resolve full-tunnel endpoint", "err", err)
			return 1
		}
		if strings.TrimSpace(resolvedTag) != "" {
			fullTunnelTag = resolvedTag
		}
	}

	updatedPath, err := config.UpdateTunEnabled(*configPath, "client", tunEnabled)
	if err != nil {
		logging.Error("xp2p client mode: update config failed", "err", err)
		return 1
	}
	if *verbose {
		if _, err := config.UpdateFullTunnelVerbose(*configPath, true); err != nil {
			logging.Error("xp2p client mode: update verbose flag failed", "err", err)
			return 1
		}
	}
	if fs.NArg() == 2 {
		if _, err := config.UpdateTunMode(*configPath, "client", tunMode); err != nil {
			logging.Error("xp2p client mode: update tun mode failed", "err", err)
			return 1
		}
	}
	if tunEnabled && tunMode == "full" && strings.TrimSpace(fullTunnelTag) != "" {
		if _, err := config.UpdateFullTunnelTag(*configPath, fullTunnelTag); err != nil {
			logging.Error("xp2p client mode: update full-tunnel tag failed", "err", err)
			return 1
		}
	}
	if err := client.ApplyModePending(client.ModeOptions{
		InstallDir:    installDir,
		ConfigDir:     configDirName,
		TunEnabled:    tunEnabled,
		TunName:       loadedCfg.Client.TunName,
		TunMTU:        loadedCfg.Client.TunMTU,
		TunAddr:       loadedCfg.Client.TunAddr,
		TunMode:       tunMode,
		FullTunnelTag: fullTunnelTag,
	}); err != nil {
		logging.Error("xp2p client mode: update pending config failed", "err", err)
		return 1
	}

	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		logging.Error("xp2p client mode: apply request failed", "err", err)
		return 1
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		logging.Error("xp2p client mode: apply request failed", "err", err)
		return 1
	}
	if err := restartClientServiceIfActive(ctx); err != nil {
		logging.Error("xp2p client mode: restart failed", "err", err)
		return 1
	}

	if fs.NArg() == 2 {
		logging.Info("xp2p client mode updated", "mode", mode, "tun_mode", tunMode, "config", updatedPath)
	} else {
		logging.Info("xp2p client mode updated", "mode", mode, "config", updatedPath)
	}
	return 0
}

func resolveClientMode(cfg config.Config) (string, error) {
	path := config.ConfigPath(layout.ClientAppliedStateFileName)
	data, err := os.ReadFile(path)
	if err == nil {
		if mode := parseModeFromState(data); mode != "" {
			return mode, nil
		}
	}
	if cfg.Client.TunEnabled {
		return "tun", nil
	}
	return "proxy", nil
}

func resolveClientTunMode(configPath string, cfg config.Config) (string, error) {
	trimmed := strings.TrimSpace(configPath)
	if trimmed == "" {
		if cfg.Client.TunMode != "" {
			return cfg.Client.TunMode, nil
		}
		trimmed = resolveConfigPath(layout.ClientConfigFileName)
	}
	loaded, err := config.Load(config.Options{
		Path:         trimmed,
		AllowInvalid: true,
	})
	if err != nil {
		return "", err
	}
	return loaded.Client.TunMode, nil
}

func parseModeFromState(data []byte) string {
	var state struct {
		Mode       string `json:"mode"`
		TunEnabled bool   `json:"tun_enabled"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return ""
	}
	mode := strings.ToLower(strings.TrimSpace(state.Mode))
	if mode == "tun" || mode == "proxy" {
		return mode
	}
	if state.TunEnabled {
		return "tun"
	}
	return "proxy"
}

func parseMode(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tun":
		return true, nil
	case "proxy":
		return false, nil
	default:
		return false, errors.New("use tun or proxy")
	}
}

func loadModeConfig(configPath string, fallback config.Config) (config.Config, error) {
	trimmed := strings.TrimSpace(configPath)
	if trimmed == "" {
		trimmed = resolveConfigPath(layout.ClientConfigFileName)
	}
	loaded, err := config.Load(config.Options{
		Path:         trimmed,
		AllowInvalid: true,
	})
	if err != nil {
		return fallback, err
	}
	return loaded, nil
}

func resolveFullTunnelBinding(installDir, configDir, tag, host, existingTag string, quiet bool) (string, string, error) {
	tagValue := strings.TrimSpace(tag)
	hostValue := strings.TrimSpace(host)
	if tagValue == "" && hostValue == "" {
		if strings.TrimSpace(existingTag) != "" {
			return strings.TrimSpace(existingTag), "", nil
		}
		if quiet {
			return "", "", errors.New("xp2p: --tag or --host is required for full-tunnel")
		}
		selection, err := promptClientRedirectBinding(installDir, configDir)
		if err != nil {
			if errors.Is(err, tagprompt.ErrEmpty) || errors.Is(err, tagprompt.ErrAborted) {
				return "", "", errors.New("xp2p: --tag or --host is required for full-tunnel")
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
			return "", "", errors.New("xp2p: client endpoint not found")
		case errors.Is(err, redirect.ErrBindingTagNotFound):
			return "", "", errors.New("xp2p: outbound tag is not registered")
		case errors.Is(err, redirect.ErrBindingTagMismatch):
			return binding.Tag, binding.Host, errors.New("xp2p: tag does not match host")
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

func resolveConfigPath(name string) string {
	live := config.ConfigPath(name)
	if _, err := os.Stat(live); err == nil {
		return live
	}
	return config.PendingConfigPath(name)
}

func restartClientServiceIfActive(ctx context.Context) error {
	ctrl := servicecontrol.Default()
	status, err := ctrl.Status(ctx, servicecontrol.RoleClient)
	if err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Warn("xp2p client mode: service status not supported; pending changes require manual restart")
			return nil
		}
		return err
	}
	if !status.Active {
		return nil
	}
	logging.Info("xp2p client mode: apply request recorded; service will restart automatically")
	return nil
}
