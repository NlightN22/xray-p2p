package servercmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/tagprompt"
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
	Host      string
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
}

func newServerRedirectCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redirect",
		Short: "Manage server redirect rules",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitError{code: 1}
		},
	}
	cmd.AddCommand(
		newServerRedirectAddCmd(cfg),
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
		Long:  "Add a server redirect rule. When --tag/--host is omitted the CLI lists reverse portals and prompts for an outbound tag.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerRedirectAdd(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Path, "path", "", "server installation directory")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "server configuration directory name or absolute path")
	flags.StringVar(&opts.CIDR, "cidr", "", "CIDR to redirect")
	flags.StringVar(&opts.Domain, "domain", "", "domain to redirect")
	flags.StringVar(&opts.Tag, "tag", "", "reverse outbound tag to route through (prompts when omitted)")
	flags.StringVar(&opts.Host, "host", "", "reverse portal host to route through")
	flags.BoolVar(&opts.Quiet, "quiet", false, "do not prompt for outbound tags")
	return cmd
}

func newServerRedirectRemoveCmd(cfg commandConfig) *cobra.Command {
	var opts serverRedirectRemoveOptions
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a server redirect rule",
		Long:  "Remove a server redirect rule. When --tag/--host is omitted the CLI lists reverse portals and prompts for an outbound tag.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerRedirectRemove(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Path, "path", "", "server installation directory")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "server configuration directory name or absolute path")
	flags.StringVar(&opts.CIDR, "cidr", "", "CIDR mapping to remove")
	flags.StringVar(&opts.Domain, "domain", "", "domain mapping to remove")
	flags.StringVar(&opts.Tag, "tag", "", "reverse outbound tag filter (prompts when omitted)")
	flags.StringVar(&opts.Host, "host", "", "reverse portal host filter")
	flags.BoolVar(&opts.Quiet, "quiet", false, "do not prompt for outbound tags")
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

	flags := cmd.Flags()
	flags.StringVar(&opts.Path, "path", "", "server installation directory")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "server configuration directory name or absolute path")
	return cmd
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

	installDir := firstNonEmpty(opts.Path, cfg.Server.InstallDir)
	configDir := firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir)

	tagValue := strings.TrimSpace(opts.Tag)
	hostValue := strings.TrimSpace(opts.Host)
	if tagValue == "" && hostValue == "" {
		if opts.Quiet {
			logging.Error("xp2p server redirect add: --tag or --host is required")
			return 2
		}
		selection, err := promptServerRedirectBinding(installDir, configDir)
		if err != nil {
			if errors.Is(err, tagprompt.ErrEmpty) || errors.Is(err, tagprompt.ErrAborted) {
				logging.Error("xp2p server redirect add: --tag or --host is required")
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
		Hostname:   hostValue,
	}
	if err := serverRedirectAddFunc(addOpts); err != nil {
		logging.Error("xp2p server redirect add failed", "err", err)
		return 1
	}

	fields := []any{"tag", strings.TrimSpace(tagValue), "host", strings.TrimSpace(hostValue)}
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
	if !hasCIDR && !hasDomain {
		logging.Error("xp2p server redirect remove: --cidr or --domain is required")
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
	if tagValue == "" && hostValue == "" {
		if opts.Quiet {
			logging.Error("xp2p server redirect remove: --tag or --host is required")
			return 2
		}
		selection, err := promptServerRedirectBinding(installDir, configDir)
		if err != nil {
			if errors.Is(err, tagprompt.ErrEmpty) || errors.Is(err, tagprompt.ErrAborted) {
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
	}
	if err := serverRedirectRemoveFunc(removeOpts); err != nil {
		logging.Error("xp2p server redirect remove failed", "err", err)
		return 1
	}

	fields := []any{"tag", strings.TrimSpace(tagValue), "host", strings.TrimSpace(hostValue)}
	if hasCIDR {
		fields = append(fields, "cidr", strings.TrimSpace(opts.CIDR))
	} else {
		fields = append(fields, "domain", strings.TrimSpace(opts.Domain))
	}
	logging.Info("xp2p server redirect removed", fields...)
	return 0
}

func runServerRedirectList(_ context.Context, cfg config.Config, opts serverRedirectListOptions) int {
	listOpts := server.RedirectListOptions{
		InstallDir: firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
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
	fmt.Fprintln(writer, "TYPE\tVALUE\tOUTBOUND TAG\tHOST")
	for _, rec := range records {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", rec.Type, rec.Value, rec.Tag, rec.Hostname)
	}
	_ = writer.Flush()
	return 0
}
