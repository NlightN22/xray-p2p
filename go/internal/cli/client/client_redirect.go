package clientcmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientRedirectCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redirect",
		Short: "Manage custom client redirects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitError{code: 1}
		},
	}
	cmd.AddCommand(
		newClientRedirectAddCmd(cfg),
		newClientRedirectRemoveCmd(cfg),
		newClientRedirectListCmd(cfg),
	)
	return cmd
}

func newClientRedirectAddCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a custom redirect rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientRedirectAdd(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	flags.String("cidr", "", "CIDR to redirect")
	flags.String("domain", "", "domain to redirect")
	flags.String("tag", "", "outbound tag to route through")
	flags.String("host", "", "client endpoint hostname to route through")
	return cmd
}

func newClientRedirectRemoveCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a redirect rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientRedirectRemove(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	flags.String("cidr", "", "CIDR mapping to remove")
	flags.String("domain", "", "domain mapping to remove")
	flags.String("tag", "", "outbound tag filter")
	flags.String("host", "", "client endpoint hostname filter")
	return cmd
}

func newClientRedirectListCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured redirect rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientRedirectList(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
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
	if strings.TrimSpace(*tag) == "" && strings.TrimSpace(*host) == "" {
		logging.Error("xp2p client redirect add: --tag or --host is required")
		return 2
	}

	opts := client.RedirectAddOptions{
		InstallDir: firstNonEmpty(*path, cfg.Client.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Client.ConfigDir),
		CIDR:       *cidr,
		Domain:     *domain,
		Tag:        *tag,
		Hostname:   *host,
	}
	if err := client.AddRedirect(opts); err != nil {
		logging.Error("xp2p client redirect add failed", "err", err)
		return 1
	}
	fields := []any{"tag", strings.TrimSpace(*tag), "host", strings.TrimSpace(*host)}
	if hasCIDR {
		fields = append(fields, "cidr", strings.TrimSpace(*cidr))
	} else {
		fields = append(fields, "domain", strings.TrimSpace(*domain))
	}
	logging.Info("xp2p client redirect added", fields...)
	return 0
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

	opts := client.RedirectRemoveOptions{
		InstallDir: firstNonEmpty(*path, cfg.Client.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Client.ConfigDir),
		CIDR:       *cidr,
		Domain:     *domain,
		Tag:        *tag,
		Hostname:   *host,
	}
	if err := client.RemoveRedirect(opts); err != nil {
		logging.Error("xp2p client redirect remove failed", "err", err)
		return 1
	}
	fields := []any{"tag", strings.TrimSpace(*tag), "host", strings.TrimSpace(*host)}
	if hasCIDR {
		fields = append(fields, "cidr", strings.TrimSpace(*cidr))
	} else {
		fields = append(fields, "domain", strings.TrimSpace(*domain))
	}
	logging.Info("xp2p client redirect removed", fields...)
	return 0
}

func runClientRedirectList(_ context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client redirect list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client redirect list: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client redirect list: unexpected arguments", "args", fs.Args())
		return 2
	}

	opts := client.RedirectListOptions{
		InstallDir: firstNonEmpty(*path, cfg.Client.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Client.ConfigDir),
	}
	records, err := client.ListRedirects(opts)
	if err != nil {
		logging.Error("xp2p client redirect list failed", "err", err)
		return 1
	}
	if len(records) == 0 {
		fmt.Println("No redirect rules configured.")
		return 0
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "TYPE\tVALUE\tOUTBOUND TAG\tHOST")
	for _, rec := range records {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", rec.Type, rec.Value, rec.Tag, rec.Hostname)
	}
	_ = writer.Flush()
	return 0
}
