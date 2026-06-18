package clientcmd

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientRedirectRemoveCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a redirect rule",
		Long:  "Remove a redirect rule. When --tag/--host is omitted the CLI lists configured endpoints and prompts for an outbound tag.",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientRedirectRemove(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.StringP("cidr", "C", "", "CIDR mapping to remove")
	flags.StringP("domain", "d", "", "domain mapping to remove")
	flags.StringP("tag", "g", "", "outbound tag filter (prompts when omitted)")
	flags.StringP("host", "H", "", "client endpoint hostname filter")
	flags.BoolP("quiet", "q", false, "do not prompt for outbound tags")
	return cmd
}

func runClientRedirectRemove(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client redirect remove", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	cidr := fs.String("cidr", "", "CIDR to remove")
	domain := fs.String("domain", "", "domain to remove")
	tag := fs.String("tag", "", "outbound tag filter")
	host := fs.String("host", "", "client endpoint host filter")
	quiet := fs.Bool("quiet", false, "do not prompt for outbound tags")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client redirect remove: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client redirect remove: unexpected arguments", "args", fs.Args())
		return 2
	}
	hasCIDR := strings.TrimSpace(*cidr) != ""
	hasDomain := strings.TrimSpace(*domain) != ""
	if !hasCIDR && !hasDomain {
		logging.Error("xp2p client redirect remove: --cidr or --domain is required")
		return 2
	}
	if hasCIDR && hasDomain {
		logging.Error("xp2p client redirect remove: specify only one of --cidr or --domain")
		return 2
	}

	installDir := firstNonEmpty(*path, cfg.Client.InstallDir)
	configDirName := firstNonEmpty(*configDir, cfg.Client.ConfigDir)
	tagValue := strings.TrimSpace(*tag)
	hostValue := strings.TrimSpace(*host)
	if tagValue == "" && hostValue == "" {
		selection, err := resolveClientBinding(clientBindingRequest{
			InstallDir: installDir,
			ConfigDir:  configDirName,
			CIDR:       *cidr,
			Domain:     *domain,
			Tag:        tagValue,
			Host:       hostValue,
			Header:     "Available matching client redirects:",
			Reader:     clientRedirectPromptReader(),
			Quiet:      *quiet,
			Matching:   true,
		})
		if err != nil {
			if clientBindingRequiredError(err) {
				logging.Error("xp2p client redirect remove: --tag or --host is required")
				return 2
			}
			logging.Error("xp2p client redirect remove: failed to enumerate endpoints", "err", err)
			return 1
		}
		tagValue = selection.Tag
		hostValue = selection.Host
	}

	opts := client.RedirectRemoveOptions{
		InstallDir: installDir,
		ConfigDir:  configDirName,
		CIDR:       *cidr,
		Domain:     *domain,
		Tag:        tagValue,
		Hostname:   hostValue,
		TunEnabled: cfg.Client.TunEnabled,
		TunName:    cfg.Client.TunName,
	}
	if err := clientRedirectRemoveFunc(opts); err != nil {
		logging.Error("xp2p client redirect remove failed", "err", err)
		return 1
	}
	fields := []any{"tag", strings.TrimSpace(tagValue), "host", strings.TrimSpace(hostValue)}
	if hasCIDR {
		fields = append(fields, "cidr", strings.TrimSpace(*cidr))
	} else {
		fields = append(fields, "domain", strings.TrimSpace(*domain))
	}
	logging.Info("xp2p client redirect removed", fields...)
	return 0
}
