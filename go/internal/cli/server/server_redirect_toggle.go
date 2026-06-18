package servercmd

import (
	"context"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/spf13/cobra"
)

type serverRedirectToggleOptions struct {
	CIDR   string
	Domain string
	Tag    string
	Host   string
	All    bool
}

func newServerRedirectDisableCmd(cfg commandConfig) *cobra.Command {
	return newServerRedirectToggleCmd(cfg, false)
}

func newServerRedirectEnableCmd(cfg commandConfig) *cobra.Command {
	return newServerRedirectToggleCmd(cfg, true)
}

func newServerRedirectToggleCmd(cfg commandConfig, enabled bool) *cobra.Command {
	var opts serverRedirectToggleOptions
	name := "disable"
	short := "Disable a server redirect rule"
	if enabled {
		name = "enable"
		short = "Enable a server redirect rule"
	}
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerRedirectToggle(commandContext(cmd), cfg(), opts, enabled)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.CIDR, "cidr", "C", "", "CIDR mapping to toggle")
	flags.StringVarP(&opts.Domain, "domain", "d", "", "domain mapping to toggle")
	flags.StringVarP(&opts.Tag, "tag", "g", "", "reverse outbound tag filter")
	flags.StringVarP(&opts.Host, "host", "H", "", "reverse portal host filter")
	flags.BoolVarP(&opts.All, "all", "a", false, "toggle all redirect rules")
	return cmd
}

func runServerRedirectToggle(_ context.Context, _ config.Config, opts serverRedirectToggleOptions, enabled bool) int {
	if !opts.All && (strings.TrimSpace(opts.CIDR) == "") == (strings.TrimSpace(opts.Domain) == "") {
		logging.Error("xp2p server redirect toggle: specify exactly one of --cidr or --domain, or use --all")
		return 2
	}
	if opts.All && (strings.TrimSpace(opts.CIDR) != "" || strings.TrimSpace(opts.Domain) != "") {
		logging.Error("xp2p server redirect toggle: --all cannot be combined with --cidr or --domain")
		return 2
	}
	if err := serverRedirectToggleFunc(server.RedirectSetEnabledOptions{
		CIDR:     opts.CIDR,
		Domain:   opts.Domain,
		Tag:      opts.Tag,
		Hostname: opts.Host,
		All:      opts.All,
		Enabled:  enabled,
	}); err != nil {
		logging.Error("xp2p server redirect toggle failed", "err", err)
		return 1
	}
	return 0
}
