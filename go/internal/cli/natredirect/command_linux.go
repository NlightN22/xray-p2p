//go:build linux

package natredirect

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/firewall"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const (
	defaultSnippet   = "/etc/nftables.d/xray-transparent.nft"
	defaultEntryDir  = "/etc/nftables.d/xray-transparent.d"
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
	cmd.AddCommand(newAddCmd(), newRemoveCmd(), newListCmd())
	return cmd
}

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add transparent redirect rules for a subnet",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := parseAddOptions(cmd)
			if err != nil {
				return err
			}
			manager := firewall.NewManager(opts.snippetPath, opts.entryDir)
			plan, err := manager.PlanAdd(opts.subnet, opts.port)
			if err != nil {
				return err
			}
			if opts.printOnly {
				printPlan(plan)
				return nil
			}
			if !opts.yes && !promptYes() {
				return exitError{code: 1}
			}
			result, err := manager.ApplyPlan(plan)
			if err != nil {
				return err
			}
			logging.Info("nat redirect applied", "subnet", opts.subnet, "port", opts.port, "backend", result.Backend)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.String("subnet", "", "destination subnet in CIDR form")
	flags.Int("port", 0, "dokodemo-door port to redirect to (auto-detected when omitted)")
	flags.Bool("print-only", false, "render firewall changes without applying them")
	flags.Bool("yes", false, "apply without interactive confirmation")
	flags.String("snippet", defaultSnippet, "nftables snippet path")
	flags.String("entry-dir", defaultEntryDir, "entry directory for nftables snippet generation")
	flags.String("inbounds", defaultInbounds, "path to inbounds.json used for auto port detection")
	return cmd
}

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove transparent redirect rules",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := parseRemoveOptions(cmd)
			if err != nil {
				return err
			}
			manager := firewall.NewManager(opts.snippetPath, opts.entryDir)
			plan, err := manager.PlanRemove(opts.subnet, opts.all)
			if err != nil {
				return err
			}
			if opts.printOnly {
				printPlan(plan)
				return nil
			}
			if !opts.yes && !promptYes() {
				return exitError{code: 1}
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
	flags.String("subnet", "", "destination subnet in CIDR form")
	flags.Bool("all", false, "remove all transparent redirects")
	flags.Bool("print-only", false, "render firewall changes without applying them")
	flags.Bool("yes", false, "apply without interactive confirmation")
	flags.String("snippet", defaultSnippet, "nftables snippet path")
	flags.String("entry-dir", defaultEntryDir, "entry directory for nftables snippet generation")
	return cmd
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List transparent redirect entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			manager := firewall.NewManager(defaultSnippet, defaultEntryDir)
			entries, err := manager.List()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("No transparent redirects configured.")
				return nil
			}
			fmt.Println("Subnet\tPort")
			for _, e := range entries {
				fmt.Printf("%s\t%d\n", e.Subnet, e.Port)
			}
			return nil
		},
	}
}

type addOptions struct {
	subnet      string
	port        int
	printOnly   bool
	yes         bool
	snippetPath string
	entryDir    string
	inbounds    string
}

type removeOptions struct {
	subnet      string
	all         bool
	printOnly   bool
	yes         bool
	snippetPath string
	entryDir    string
}

func parseAddOptions(cmd *cobra.Command) (addOptions, error) {
	subnet, _ := cmd.Flags().GetString("subnet")
	port, _ := cmd.Flags().GetInt("port")
	printOnly, _ := cmd.Flags().GetBool("print-only")
	yes, _ := cmd.Flags().GetBool("yes")
	snippet, _ := cmd.Flags().GetString("snippet")
	entryDir, _ := cmd.Flags().GetString("entry-dir")
	inbounds, _ := cmd.Flags().GetString("inbounds")
	if subnet == "" {
		return addOptions{}, fmt.Errorf("nat-redirect add: --subnet is required")
	}
	if port == 0 {
		auto, err := firewall.DetectDokodemoPorts(inbounds)
		if err != nil {
			return addOptions{}, err
		}
		if len(auto) == 0 {
			return addOptions{}, fmt.Errorf("nat-redirect add: no dokodemo-door ports found in %s", inbounds)
		}
		if len(auto) == 1 {
			port = auto[0]
		} else {
			fmt.Printf("Detected dokodemo-door ports: %v\n", auto)
			selected, err := promptSelectPort(auto)
			if err != nil {
				return addOptions{}, err
			}
			port = selected
		}
	}
	return addOptions{
		subnet:      subnet,
		port:        port,
		printOnly:   printOnly,
		yes:         yes,
		snippetPath: fallback(snippet, defaultSnippet),
		entryDir:    fallback(entryDir, defaultEntryDir),
		inbounds:    fallback(inbounds, defaultInbounds),
	}, nil
}

func parseRemoveOptions(cmd *cobra.Command) (removeOptions, error) {
	subnet, _ := cmd.Flags().GetString("subnet")
	all, _ := cmd.Flags().GetBool("all")
	printOnly, _ := cmd.Flags().GetBool("print-only")
	yes, _ := cmd.Flags().GetBool("yes")
	snippet, _ := cmd.Flags().GetString("snippet")
	entryDir, _ := cmd.Flags().GetString("entry-dir")
	if subnet == "" && !all {
		return removeOptions{}, fmt.Errorf("nat-redirect remove: --subnet or --all is required")
	}
	return removeOptions{
		subnet:      subnet,
		all:         all,
		printOnly:   printOnly,
		yes:         yes,
		snippetPath: fallback(snippet, defaultSnippet),
		entryDir:    fallback(entryDir, defaultEntryDir),
	}, nil
}

func promptYes() bool {
	fmt.Print(promptYesMessage)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

func promptSelectPort(ports []int) (int, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Select port number: ")
		raw, _ := reader.ReadString('\n')
		raw = strings.TrimSpace(raw)
		val, err := strconv.Atoi(raw)
		if err == nil {
			for _, p := range ports {
				if p == val {
					return val, nil
				}
			}
		}
		fmt.Println("Invalid selection.")
	}
}

func printPlan(plan firewall.Plan) {
	if len(plan.Snippet) > 0 {
		fmt.Printf("Planned nftables snippet (%s):\n%s\n", plan.SnippetPath, plan.Snippet)
	}
	if len(plan.IPTables) > 0 {
		fmt.Println("Planned iptables commands:")
		for _, line := range plan.IPTables {
			fmt.Println(line)
		}
	}
	if plan.EntryPath != "" {
		fmt.Printf("Entry file would be written to %s\n", plan.EntryPath)
	}
}

func removeTarget(opts removeOptions) string {
	if opts.all {
		return "all"
	}
	return opts.subnet
}

func fallback(value, def string) string {
	trim := strings.TrimSpace(value)
	if trim == "" {
		return def
	}
	return trim
}

type exitError struct {
	code int
}

func (e exitError) Error() string {
	return fmt.Sprintf("exit %d", e.code)
}

func (e exitError) ExitCode() int {
	return e.code
}
