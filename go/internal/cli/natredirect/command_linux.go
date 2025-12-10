//go:build linux

package natredirect

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
			if !opts.quiet && !promptYes() {
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
	flags.Bool("quiet", false, "avoid interactive prompts when auto-selecting dokodemo port")
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
	snippetPath string
	entryDir    string
	inbounds    string
	quiet       bool
}

type removeOptions struct {
	subnet      string
	all         bool
	printOnly   bool
	snippetPath string
	entryDir    string
}

func parseAddOptions(cmd *cobra.Command) (addOptions, error) {
	subnet, _ := cmd.Flags().GetString("subnet")
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
	if subnet == "" {
		return addOptions{}, fmt.Errorf("nat-redirect add: --subnet is required")
	}
	if port == 0 {
		candidates, err := autodetectPorts(detectInbounds, quiet)
		if err != nil {
			return addOptions{}, err
		}
		port = candidates[0]
	}
	return addOptions{
		subnet:      subnet,
		port:        port,
		printOnly:   printOnly,
		snippetPath: fallback(snippet, defaultSnippet),
		entryDir:    fallback(entryDir, defaultEntryDir),
		inbounds:    fallback(inbounds, defaultInbounds),
		quiet:       quiet,
	}, nil
}

func parseRemoveOptions(cmd *cobra.Command) (removeOptions, error) {
	subnet, _ := cmd.Flags().GetString("subnet")
	all, _ := cmd.Flags().GetBool("all")
	printOnly, _ := cmd.Flags().GetBool("print-only")
	snippet, _ := cmd.Flags().GetString("snippet")
	entryDir, _ := cmd.Flags().GetString("entry-dir")
	if subnet == "" && !all {
		return removeOptions{}, fmt.Errorf("nat-redirect remove: --subnet or --all is required")
	}
	return removeOptions{
		subnet:      subnet,
		all:         all,
		printOnly:   printOnly,
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

func detectDefaultPaths() (string, string) {
	candidates := []string{
		"/etc/nftables.d",
		filepath.Join(layout.UnixConfigRoot, "nftables"),
	}
	for _, base := range candidates {
		dir := strings.TrimSpace(base)
		if dir == "" {
			continue
		}
		// Try to ensure the directory exists so later writes succeed quietly.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		return filepath.Join(dir, "xray-transparent.nft"), filepath.Join(dir, "xray-transparent.d")
	}
	if commandExists("fw4") {
		return "/etc/nftables.d/xray-transparent.nft", "/etc/nftables.d/xray-transparent.d"
	}
	return "/etc/nftables.d/xray-transparent.nft", "/etc/nftables.d/xray-transparent.d"
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func autodetectPorts(inboundsFlag string, quiet bool) ([]int, error) {
	seen := map[int]struct{}{}
	var ports []int
	candidates := []string{strings.TrimSpace(inboundsFlag)}
	if strings.TrimSpace(inboundsFlag) == "" {
		candidates = []string{
			defaultInbounds,
			layout.UnixConfigRoot + "/" + layout.ServerConfigDir + "/inbounds.json",
		}
	}
	for _, path := range candidates {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		if info, err := os.Stat(trimmed); err != nil || info.IsDir() {
			continue
		}
		detected, err := firewall.DetectDokodemoPorts(trimmed, true)
		if err != nil {
			continue
		}
		for _, p := range detected {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			ports = append(ports, p)
		}
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("nat-redirect add: no dokodemo-door ports found")
	}
	if len(ports) == 1 {
		return ports, nil
	}
	if quiet {
		return ports, nil
	}
	fmt.Printf("Detected dokodemo-door ports: %v\n", ports)
	selected, err := promptSelectPort(ports)
	if err != nil {
		return nil, err
	}
	return []int{selected}, nil
}
