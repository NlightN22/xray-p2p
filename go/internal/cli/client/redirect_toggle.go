package clientcmd

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/spf13/cobra"
)

func newClientRedirectDisableCmd(cfg commandConfig) *cobra.Command {
	return newClientRedirectToggleCmd(cfg, false)
}

func newClientRedirectEnableCmd(cfg commandConfig) *cobra.Command {
	return newClientRedirectToggleCmd(cfg, true)
}

func newClientRedirectToggleCmd(cfg commandConfig, enabled bool) *cobra.Command {
	name := "disable"
	short := "Disable a redirect rule"
	if enabled {
		name = "enable"
		short = "Enable a redirect rule"
	}
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientRedirectToggle(commandContext(cmd), cfg(), forwarded, enabled)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringP("cidr", "C", "", "CIDR mapping to toggle")
	flags.StringP("domain", "d", "", "domain mapping to toggle")
	flags.StringP("tag", "g", "", "outbound tag filter")
	flags.StringP("host", "H", "", "client endpoint hostname filter")
	flags.BoolP("all", "a", false, "toggle all redirect rules")
	return cmd
}

func runClientRedirectToggle(_ context.Context, _ config.Config, args []string, enabled bool) int {
	fs := flag.NewFlagSet("xp2p client redirect toggle", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	cidr := fs.String("cidr", "", "CIDR mapping to toggle")
	domain := fs.String("domain", "", "domain mapping to toggle")
	tag := fs.String("tag", "", "outbound tag filter")
	host := fs.String("host", "", "client endpoint hostname filter")
	all := fs.Bool("all", false, "toggle all redirect rules")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client redirect toggle: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client redirect toggle: unexpected arguments", "args", fs.Args())
		return 2
	}
	if !*all && (strings.TrimSpace(*cidr) == "") == (strings.TrimSpace(*domain) == "") {
		logging.Error("xp2p client redirect toggle: specify exactly one of --cidr or --domain, or use --all")
		return 2
	}
	if *all && (strings.TrimSpace(*cidr) != "" || strings.TrimSpace(*domain) != "") {
		logging.Error("xp2p client redirect toggle: --all cannot be combined with --cidr or --domain")
		return 2
	}
	if err := clientRedirectToggleFunc(client.RedirectSetEnabledOptions{
		CIDR:     *cidr,
		Domain:   *domain,
		Tag:      *tag,
		Hostname: *host,
		All:      *all,
		Enabled:  enabled,
	}); err != nil {
		logging.Error("xp2p client redirect toggle failed", "err", err)
		return 1
	}
	return 0
}
