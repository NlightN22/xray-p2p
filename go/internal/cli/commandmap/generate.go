package commandmap

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/NlightN22/xray-p2p/go/internal/cli/commandmeta"
)

// Generate writes compact command map files derived from a Cobra tree.
func Generate(root *cobra.Command, dir string) error {
	if root == nil {
		return fmt.Errorf("root command is nil")
	}
	oldSort := cobra.EnableCommandSorting
	cobra.EnableCommandSorting = false
	defer func() {
		cobra.EnableCommandSorting = oldSort
	}()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create command map directory: %w", err)
	}

	for _, cmd := range documentRoots(root) {
		var out bytes.Buffer
		renderCommandTree(&out, root, cmd)
		path := filepath.Join(dir, fileName(cmd))
		if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func documentRoots(root *cobra.Command) []*cobra.Command {
	roots := []*cobra.Command{root}
	for _, cmd := range visibleCommands(root) {
		roots = append(roots, cmd)
	}
	return roots
}

func renderCommandTree(out *bytes.Buffer, root, cmd *cobra.Command) {
	fmt.Fprintf(out, "# %s\n\n", cmd.CommandPath())
	if cmd == root {
		renderGlobalOptions(out, root)
	}
	fmt.Fprintln(out, "## Command tree")
	fmt.Fprintln(out)
	if cmd == root {
		renderCommand(out, root, cmd, 1)
		return
	}
	renderCommand(out, root, cmd, -1)
}

func renderGlobalOptions(out *bytes.Buffer, root *cobra.Command) {
	flags := collectFlags(root.PersistentFlags())
	fmt.Fprintln(out, "## Global options (apply to all commands)")
	fmt.Fprintln(out, "- --help, -h show help for command")
	for _, flag := range flags {
		fmt.Fprintf(out, "- %s\n", formatFlag(flag))
	}
	fmt.Fprintln(out)
}

func renderCommand(out *bytes.Buffer, root, cmd *cobra.Command, remainingDepth int) {
	fmt.Fprintln(out, commandUseLine(cmd))
	if short := strings.TrimSpace(cmd.Short); short != "" {
		fmt.Fprintf(out, "  Summary: %s\n", short)
	}
	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(out, "  Aliases: %s\n", strings.Join(cmd.Aliases, ", "))
	}
	if len(cmd.ValidArgs) > 0 {
		fmt.Fprintf(out, "  Valid args: %s\n", strings.Join(cmd.ValidArgs, ", "))
	}
	if behavior := defaultBehavior(cmd); behavior != "" {
		fmt.Fprintf(out, "  Default behavior: %s\n", behavior)
	}

	children := visibleCommands(cmd)
	if len(children) > 0 {
		names := make([]string, 0, len(children))
		for _, child := range children {
			names = append(names, commandName(child))
		}
		fmt.Fprintf(out, "  Subcommands: %s\n", strings.Join(names, ", "))
	}

	renderOptions(out, root, cmd)
	fmt.Fprintln(out)

	if remainingDepth == 0 {
		return
	}
	nextDepth := remainingDepth
	if remainingDepth > 0 {
		nextDepth--
	}
	for _, child := range children {
		renderCommand(out, root, child, nextDepth)
	}
}

func defaultBehavior(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	return strings.TrimSpace(cmd.Annotations[commandmeta.DefaultBehavior])
}

func renderOptions(out *bytes.Buffer, root, cmd *cobra.Command) {
	flags := collectFlags(cmd.LocalFlags())
	hasInherited := cmd != root && len(collectFlags(cmd.InheritedFlags())) > 0
	if cmd == root || len(flags) == 0 && !hasInherited {
		return
	}

	fmt.Fprintln(out, "Options:")
	if hasInherited {
		fmt.Fprintln(out, "Includes: inherited options")
	}
	for _, flag := range flags {
		fmt.Fprintf(out, "- %s\n", formatFlag(flag))
	}
}

func visibleCommands(cmd *cobra.Command) []*cobra.Command {
	children := make([]*cobra.Command, 0)
	for _, child := range cmd.Commands() {
		if child.Hidden || child.Name() == "help" {
			continue
		}
		children = append(children, child)
	}
	return children
}

func collectFlags(flags *pflag.FlagSet) []*pflag.Flag {
	if flags == nil {
		return nil
	}
	list := make([]*pflag.Flag, 0)
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden || flag.Name == "help" {
			return
		}
		list = append(list, flag)
	})
	return list
}

func formatFlag(flag *pflag.Flag) string {
	name := "--" + flag.Name
	if flag.Shorthand != "" {
		name += ", -" + flag.Shorthand
	}
	if placeholder := flagPlaceholder(flag); placeholder != "" {
		name += " " + placeholder
	}
	if isRequired(flag) {
		name += " (required)"
	}
	usage := strings.TrimSpace(flag.Usage)
	if usage == "" {
		return name
	}
	return fmt.Sprintf("%s %s", name, usage)
}

func flagPlaceholder(flag *pflag.Flag) string {
	if flag.Value.Type() == "bool" {
		return ""
	}
	switch flag.Name {
	case "path", "cert", "output", "input", "config", "xray-bin", "log-file":
		return "<path>"
	case "key", "password":
		return "<password>"
	case "cert-store":
		return "<ref>"
	case "id", "new-id", "tag", "user", "remark", "endpoint":
		return "<id>"
	case "link":
		return "<link>"
	case "dir", "config-dir", "config-root", "install-dir":
		return "<dir>"
	case "host", "sni", "domain", "server-name":
		return "<host>"
	case "port", "listen-port", "base-port", "trojan-port", "diag-service-port":
		return "<port>"
	case "listen", "xray-api":
		return "<host:port>"
	case "target":
		return "<host:port>"
	case "cidr":
		return "<cidr>"
	case "proto":
		return "<proto>"
	case "mode", "tun-mode", "diag-service-mode", "xray-stats-format":
		return "<mode>"
	case "timeout", "interval", "ttl", "restart-delay":
		return "<duration>"
	case "count", "index", "max-restarts":
		return "<n>"
	}
	return "<" + flag.Value.Type() + ">"
}

func isRequired(flag *pflag.Flag) bool {
	values := flag.Annotations[cobra.BashCompOneRequiredFlag]
	for _, value := range values {
		if value == "true" {
			return true
		}
	}
	return false
}

func commandUseLine(cmd *cobra.Command) string {
	parts := make([]string, 0)
	for current := cmd; current != nil; current = current.Parent() {
		parts = append(parts, strings.TrimSpace(current.Use))
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, " ")
}

func commandName(cmd *cobra.Command) string {
	name := strings.TrimSpace(cmd.Name())
	if name == "" {
		name = strings.Fields(cmd.Use)[0]
	}
	return name
}

func fileName(cmd *cobra.Command) string {
	path := strings.ReplaceAll(cmd.CommandPath(), " ", "_")
	path = strings.ReplaceAll(path, "-", "_")
	return path + ".md"
}
