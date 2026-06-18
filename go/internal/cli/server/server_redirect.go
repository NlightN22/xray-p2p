package servercmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/commandmeta"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

type serverRedirectAddOptions struct {
	Path      string
	ConfigDir string
	CIDR      string
	Domain    string
	Tag       string
	User      string
	Host      string
	NoRoutes  bool
	Quiet     bool
}

type serverRedirectRemoveOptions struct {
	Path      string
	ConfigDir string
	CIDR      string
	Domain    string
	Tag       string
	Host      string
	Quiet     bool
}

type serverRedirectListOptions struct {
	Path      string
	ConfigDir string
	Pending   bool
}

func newServerRedirectCmd(cfg commandConfig) *cobra.Command {
	var opts serverRedirectListOptions
	cmd := &cobra.Command{
		Use:   "redirect",
		Short: "Manage server redirect rules",
		Annotations: map[string]string{
			commandmeta.DefaultBehavior: "list server redirect rules",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerRedirectList(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}
	bindServerRedirectListFlags(cmd, &opts)
	cmd.AddCommand(
		newServerRedirectAddCmd(cfg),
		newServerRedirectDisableCmd(cfg),
		newServerRedirectEnableCmd(cfg),
		newServerRedirectRemoveCmd(cfg),
		newServerRedirectListCmd(cfg),
	)
	return cmd
}

func newServerRedirectAddCmd(cfg commandConfig) *cobra.Command {
	var opts serverRedirectAddOptions
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a server redirect rule",
		Long:  "Add a server redirect rule. When --tag/--user/--host is omitted the CLI lists reverse portals and prompts for an outbound tag.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerRedirectAdd(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.StringVarP(&opts.CIDR, "cidr", "C", "", "CIDR to redirect")
	flags.StringVarP(&opts.Domain, "domain", "d", "", "domain to redirect")
	flags.StringVarP(&opts.Tag, "tag", "g", "", "reverse outbound tag to route through (prompts when omitted)")
	flags.StringVarP(&opts.User, "user", "u", "", "reverse user to route through")
	flags.StringVarP(&opts.Host, "host", "H", "", "reverse portal host to route through")
	flags.BoolVarP(&opts.NoRoutes, "no-routes", "N", false, "do not add OS routes for CIDR redirects")
	flags.BoolVarP(&opts.Quiet, "quiet", "q", false, "do not prompt for outbound tags")
	return cmd
}

func newServerRedirectRemoveCmd(cfg commandConfig) *cobra.Command {
	var opts serverRedirectRemoveOptions
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a server redirect rule",
		Long:  "Remove a server redirect rule. When --tag/--host is omitted for a CIDR or domain, the CLI lists reverse portals and prompts for an outbound tag.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerRedirectRemove(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.StringVarP(&opts.CIDR, "cidr", "C", "", "CIDR mapping to remove")
	flags.StringVarP(&opts.Domain, "domain", "d", "", "domain mapping to remove")
	flags.StringVarP(&opts.Tag, "tag", "g", "", "reverse outbound tag filter or tag-only cleanup selector")
	flags.StringVarP(&opts.Host, "host", "H", "", "reverse portal host filter")
	flags.BoolVarP(&opts.Quiet, "quiet", "q", false, "do not prompt for outbound tags")
	return cmd
}

func newServerRedirectListCmd(cfg commandConfig) *cobra.Command {
	var opts serverRedirectListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List server redirect rules",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerRedirectList(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	bindServerRedirectListFlags(cmd, &opts)
	return cmd
}

func bindServerRedirectListFlags(cmd *cobra.Command, opts *serverRedirectListOptions) {
	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.BoolVarP(&opts.Pending, "pending", "y", false, "list pending configuration")
}

func runServerRedirectAdd(_ context.Context, cfg config.Config, opts serverRedirectAddOptions) int {
	hasCIDR := strings.TrimSpace(opts.CIDR) != ""
	hasDomain := strings.TrimSpace(opts.Domain) != ""
	if !hasCIDR && !hasDomain {
		logging.Error("xp2p server redirect add: --cidr or --domain is required")
		return 2
	}
	if hasCIDR && hasDomain {
		logging.Error("xp2p server redirect add: specify only one of --cidr or --domain")
		return 2
	}
	if opts.NoRoutes && !hasCIDR {
		logging.Error("xp2p server redirect add: --no-routes requires --cidr")
		return 2
	}

	installDir := firstNonEmpty(opts.Path, cfg.Server.InstallDir)
	configDir := firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir)

	tagValue := strings.TrimSpace(opts.Tag)
	userValue := strings.TrimSpace(opts.User)
	hostValue := strings.TrimSpace(opts.Host)
	if tagValue != "" && userValue != "" {
		logging.Error("xp2p server redirect add: specify only one of --tag or --user")
		return 2
	}
	if tagValue == "" && userValue == "" && hostValue == "" {
		selection, err := resolveServerBinding(serverBindingRequest{
			InstallDir: installDir,
			ConfigDir:  configDir,
			Tag:        tagValue,
			Host:       hostValue,
			Header:     "Available reverse portals:",
			Reader:     serverRedirectPromptReader(),
			Quiet:      opts.Quiet,
		})
		if err != nil {
			if serverBindingRequiredError(err) {
				logging.Error("xp2p server redirect add: --tag, --user, or --host is required")
				return 2
			}
			logging.Error("xp2p server redirect add: failed to enumerate reverse portals", "err", err)
			return 1
		}
		tagValue = selection.Tag
		hostValue = selection.Host
	}

	addOpts := server.RedirectAddOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		CIDR:       opts.CIDR,
		Domain:     opts.Domain,
		Tag:        tagValue,
		User:       userValue,
		Hostname:   hostValue,
		NoRoutes:   opts.NoRoutes,
		TunEnabled: cfg.Server.TunEnabled,
		TunName:    cfg.Server.TunName,
	}
	if err := serverRedirectAddFunc(addOpts); err != nil {
		logging.Error("xp2p server redirect add failed", "err", err)
		return 1
	}

	fields := []any{"tag", strings.TrimSpace(tagValue), "user", strings.TrimSpace(userValue), "host", strings.TrimSpace(hostValue)}
	if hasCIDR {
		fields = append(fields, "cidr", strings.TrimSpace(opts.CIDR))
	} else {
		fields = append(fields, "domain", strings.TrimSpace(opts.Domain))
	}
	logging.Info("xp2p server redirect added", fields...)
	return 0
}

func runServerRedirectRemove(_ context.Context, cfg config.Config, opts serverRedirectRemoveOptions) int {
	hasCIDR := strings.TrimSpace(opts.CIDR) != ""
	hasDomain := strings.TrimSpace(opts.Domain) != ""
	hasTag := strings.TrimSpace(opts.Tag) != ""
	if !hasCIDR && !hasDomain && !hasTag {
		logging.Error("xp2p server redirect remove: --cidr, --domain, or --tag is required")
		return 2
	}
	if hasCIDR && hasDomain {
		logging.Error("xp2p server redirect remove: specify only one of --cidr or --domain")
		return 2
	}

	installDir := firstNonEmpty(opts.Path, cfg.Server.InstallDir)
	configDir := firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir)
	tagValue := strings.TrimSpace(opts.Tag)
	hostValue := strings.TrimSpace(opts.Host)
	if (hasCIDR || hasDomain) && tagValue == "" && hostValue == "" {
		selection, err := resolveServerBinding(serverBindingRequest{
			InstallDir: installDir,
			ConfigDir:  configDir,
			CIDR:       opts.CIDR,
			Domain:     opts.Domain,
			Tag:        tagValue,
			Host:       hostValue,
			Header:     "Available matching server redirects:",
			Reader:     serverRedirectPromptReader(),
			Quiet:      opts.Quiet,
			Matching:   true,
		})
		if err != nil {
			if serverBindingRequiredError(err) {
				logging.Error("xp2p server redirect remove: --tag or --host is required")
				return 2
			}
			logging.Error("xp2p server redirect remove: failed to enumerate reverse portals", "err", err)
			return 1
		}
		tagValue = selection.Tag
		hostValue = selection.Host
	}

	removeOpts := server.RedirectRemoveOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		CIDR:       opts.CIDR,
		Domain:     opts.Domain,
		Tag:        tagValue,
		Hostname:   hostValue,
		TunEnabled: cfg.Server.TunEnabled,
		TunName:    cfg.Server.TunName,
	}
	if err := serverRedirectRemoveFunc(removeOpts); err != nil {
		logging.Error("xp2p server redirect remove failed", "err", err)
		return 1
	}

	fields := []any{"tag", strings.TrimSpace(tagValue), "host", strings.TrimSpace(hostValue)}
	if hasCIDR {
		fields = append(fields, "cidr", strings.TrimSpace(opts.CIDR))
	} else if hasDomain {
		fields = append(fields, "domain", strings.TrimSpace(opts.Domain))
	}
	logging.Info("xp2p server redirect removed", fields...)
	return 0
}

func runServerRedirectList(_ context.Context, cfg config.Config, opts serverRedirectListOptions) int {
	listOpts := server.RedirectListOptions{
		InstallDir: firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		Pending:    opts.Pending,
	}
	records, err := serverRedirectListFunc(listOpts)
	if err != nil {
		logging.Error("xp2p server redirect list failed", "err", err)
		return 1
	}
	if len(records) == 0 {
		fmt.Println("No server redirect rules configured.")
		return 0
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "TYPE\tVALUE\tOUTBOUND TAG\tHOST\tSTATE")
	for _, rec := range records {
		state := "enabled"
		if rec.Disabled {
			state = "disabled"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", rec.Type, rec.Value, rec.Tag, rec.Hostname, state)
	}
	_ = writer.Flush()
	return 0
}
