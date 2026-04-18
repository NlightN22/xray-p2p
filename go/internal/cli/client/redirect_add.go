package clientcmd

import (
	"context"
	"errors"
	"flag"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/tagprompt"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientRedirectAddCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a custom redirect rule",
		Long:  "Add a custom redirect rule. When --tag/--host is omitted the CLI lists configured endpoints and prompts for an outbound tag.",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientRedirectAdd(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.StringP("cidr", "C", "", "CIDR to redirect")
	flags.StringP("domain", "d", "", "domain to redirect")
	flags.StringP("tag", "g", "", "outbound tag to route through (prompts when omitted)")
	flags.StringP("host", "H", "", "client endpoint hostname to route through")
	flags.BoolP("no-routes", "N", false, "do not add OS routes for CIDR redirects")
	flags.BoolP("quiet", "q", false, "do not prompt for outbound tags")
	return cmd
}

func runClientRedirectAdd(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client redirect add", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	cidr := fs.String("cidr", "", "CIDR to redirect")
	domain := fs.String("domain", "", "domain to redirect")
	tag := fs.String("tag", "", "outbound tag to use")
	host := fs.String("host", "", "client endpoint hostname")
	noRoutes := fs.Bool("no-routes", false, "do not add OS routes for CIDR redirects")
	quiet := fs.Bool("quiet", false, "do not prompt for outbound tags")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client redirect add: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client redirect add: unexpected arguments", "args", fs.Args())
		return 2
	}
	hasCIDR := strings.TrimSpace(*cidr) != ""
	hasDomain := strings.TrimSpace(*domain) != ""
	if !hasCIDR && !hasDomain {
		logging.Error("xp2p client redirect add: --cidr or --domain is required")
		return 2
	}
	if hasCIDR && hasDomain {
		logging.Error("xp2p client redirect add: specify only one of --cidr or --domain")
		return 2
	}
	if *noRoutes && !hasCIDR {
		logging.Error("xp2p client redirect add: --no-routes requires --cidr")
		return 2
	}

	installDir := firstNonEmpty(*path, cfg.Client.InstallDir)
	configDirName := firstNonEmpty(*configDir, cfg.Client.ConfigDir)

	tagValue := strings.TrimSpace(*tag)
	hostValue := strings.TrimSpace(*host)
	if tagValue == "" && hostValue == "" {
		if *quiet {
			logging.Error("xp2p client redirect add: --tag or --host is required")
			return 2
		}
		selection, err := promptClientRedirectBinding(installDir, configDirName)
		if err != nil {
			if errors.Is(err, tagprompt.ErrEmpty) || errors.Is(err, tagprompt.ErrAborted) {
				logging.Error("xp2p client redirect add: --tag or --host is required")
				return 2
			}
			logging.Error("xp2p client redirect add: failed to enumerate endpoints", "err", err)
			return 1
		}
		tagValue = selection.Tag
		hostValue = selection.Host
	}

	opts := client.RedirectAddOptions{
		InstallDir: installDir,
		ConfigDir:  configDirName,
		CIDR:       *cidr,
		Domain:     *domain,
		Tag:        tagValue,
		Hostname:   hostValue,
		NoRoutes:   *noRoutes,
		TunEnabled: cfg.Client.TunEnabled,
		TunName:    cfg.Client.TunName,
	}
	if err := clientRedirectAddFunc(opts); err != nil {
		logging.Error("xp2p client redirect add failed", "err", err)
		return 1
	}
	fields := []any{"tag", strings.TrimSpace(tagValue), "host", strings.TrimSpace(hostValue)}
	if hasCIDR {
		fields = append(fields, "cidr", strings.TrimSpace(*cidr))
	} else {
		fields = append(fields, "domain", strings.TrimSpace(*domain))
	}
	logging.Info("xp2p client redirect added", fields...)
	return 0
}
