//go:build linux

package natredirect

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/firewall"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

var (
	defaultSnippet  string
	defaultEntryDir string
)

func init() {
	defaultSnippet, defaultEntryDir = detectDefaultPaths()
}

const (
	defaultInbounds  = layout.UnixConfigRoot + "/" + layout.ClientConfigDir + "/inbounds.json"
	promptYesMessage = "This will change local firewall rules for transparent redirect. Continue? [y/N]: "
)

func NewCommand(cfg func() config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nat-redirect",
		Short: "Manage transparent NAT redirect rules (Linux only)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitError{code: 1}
		},
	}
	cmd.AddCommand(newAddCmd(cfg), newRemoveCmd(cfg), newListCmd(cfg))
	return cmd
}

func newAddCmd(cfg func() config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add transparent redirect rules for a CIDR",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := parseAddOptions(cmd)
			if err != nil {
				return err
			}
			if err := ensureProxyMode(cfg(), opts.inbounds); err != nil {
				return err
			}
			manager := firewall.NewManager(opts.snippetPath, opts.entryDir)
			plan, err := manager.PlanAdd(opts.cidr, opts.port)
			if err != nil {
				return err
			}
			if opts.printOnly {
				printPlan(plan)
				return nil
			}
			if !opts.quiet && !promptYes() {
				return exitError{code: 1}
			}
			result, err := manager.ApplyPlan(plan)
			if err != nil {
				return err
			}
			logging.Info("nat redirect applied", "cidr", opts.cidr, "port", opts.port, "backend", result.Backend)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.String("cidr", "", "destination CIDR")
	flags.Int("port", 0, "dokodemo-door port to redirect to (auto-detected when omitted)")
	flags.Bool("print-only", false, "render firewall changes without applying them")
	flags.Bool("quiet", false, "avoid interactive prompts when auto-selecting dokodemo port")
	flags.String("snippet", defaultSnippet, "nftables snippet path")
	flags.String("entry-dir", defaultEntryDir, "entry directory for nftables snippet generation")
	flags.String("inbounds", defaultInbounds, "path to inbounds.json used for auto port detection")
	return cmd
}

func newRemoveCmd(cfg func() config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove transparent redirect rules",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := parseRemoveOptions(cmd)
			if err != nil {
				return err
			}
			if err := ensureProxyMode(cfg(), defaultInbounds); err != nil {
				return err
			}
			manager := firewall.NewManager(opts.snippetPath, opts.entryDir)
			plan, err := manager.PlanRemove(opts.cidr, opts.all)
			if err != nil {
				return err
			}
			if opts.printOnly {
				printPlan(plan)
				return nil
			}
			result, err := manager.ApplyPlan(plan)
			if err != nil {
				return err
			}
			logging.Info("nat redirect removed", "target", removeTarget(opts))
			_ = result
			return nil
		},
	}
	flags := cmd.Flags()
	flags.String("cidr", "", "destination CIDR")
	flags.Bool("all", false, "remove all transparent redirects")
	flags.Bool("print-only", false, "render firewall changes without applying them")
	flags.String("snippet", defaultSnippet, "nftables snippet path")
	flags.String("entry-dir", defaultEntryDir, "entry directory for nftables snippet generation")
	return cmd
}

func newListCmd(cfg func() config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List transparent redirect entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ensureProxyMode(cfg(), defaultInbounds); err != nil {
				return err
			}
			manager := firewall.NewManager(defaultSnippet, defaultEntryDir)
			entries, err := manager.List()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("No transparent redirects configured.")
				return nil
			}
			fmt.Println("CIDR\tPort")
			for _, e := range entries {
				fmt.Printf("%s\t%d\n", e.CIDR, e.Port)
			}
			return nil
		},
	}
}

type addOptions struct {
	cidr        string
	port        int
	printOnly   bool
	snippetPath string
	entryDir    string
	inbounds    string
	quiet       bool
}

type removeOptions struct {
	cidr        string
	all         bool
	printOnly   bool
	snippetPath string
	entryDir    string
}

func parseAddOptions(cmd *cobra.Command) (addOptions, error) {
	cidr, _ := cmd.Flags().GetString("cidr")
	port, _ := cmd.Flags().GetInt("port")
	printOnly, _ := cmd.Flags().GetBool("print-only")
	snippet, _ := cmd.Flags().GetString("snippet")
	entryDir, _ := cmd.Flags().GetString("entry-dir")
	inbounds, _ := cmd.Flags().GetString("inbounds")
	quiet, _ := cmd.Flags().GetBool("quiet")
	detectInbounds := inbounds
	if strings.TrimSpace(inbounds) == defaultInbounds {
		detectInbounds = ""
	}
	if cidr == "" {
		return addOptions{}, fmt.Errorf("nat-redirect add: --cidr is required")
	}
	if port == 0 {
		candidates, err := autodetectPorts(detectInbounds, quiet)
		if err != nil {
			return addOptions{}, err
		}
		port = candidates[0]
	}
	return addOptions{
		cidr:        cidr,
		port:        port,
		printOnly:   printOnly,
		snippetPath: fallback(snippet, defaultSnippet),
		entryDir:    fallback(entryDir, defaultEntryDir),
		inbounds:    fallback(inbounds, defaultInbounds),
		quiet:       quiet,
	}, nil
}

func parseRemoveOptions(cmd *cobra.Command) (removeOptions, error) {
	cidr, _ := cmd.Flags().GetString("cidr")
	all, _ := cmd.Flags().GetBool("all")
	printOnly, _ := cmd.Flags().GetBool("print-only")
	snippet, _ := cmd.Flags().GetString("snippet")
	entryDir, _ := cmd.Flags().GetString("entry-dir")
	if cidr == "" && !all {
		return removeOptions{}, fmt.Errorf("nat-redirect remove: --cidr or --all is required")
	}
	return removeOptions{
		cidr:        cidr,
		all:         all,
		printOnly:   printOnly,
		snippetPath: fallback(snippet, defaultSnippet),
		entryDir:    fallback(entryDir, defaultEntryDir),
	}, nil
}
