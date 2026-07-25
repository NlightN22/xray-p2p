package clientcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

func bindClientMutationResults(root *cobra.Command) {
	specs := map[string]func(*cobra.Command, []string) string{
		"disable":              clientMutationFirstArg,
		"enable":               clientMutationFirstArg,
		"forward add":          clientMutationFlag("target"),
		"forward remove":       clientMutationFlag("listen-port"),
		"redirect add":         clientMutationRedirectEntity,
		"redirect disable":     clientMutationRedirectEntity,
		"redirect enable":      clientMutationRedirectEntity,
		"redirect remove":      clientMutationRedirectEntity,
		"reverse disable":      clientMutationFirstArg,
		"reverse enable":       clientMutationFirstArg,
		"subscription add":     clientMutationFirstArg,
		"subscription refresh": clientMutationFirstArg,
		"subscription remove":  clientMutationFirstArg,
		"update":               clientMutationFirstArg,
	}
	bindClientMutationSpecs(root, specs)
}

func bindClientMutationSpecs(
	root *cobra.Command,
	specs map[string]func(*cobra.Command, []string) string,
) {
	seen := make(map[string]bool, len(specs))
	var visit func(*cobra.Command, []string)
	visit = func(cmd *cobra.Command, parents []string) {
		current := append(parents, cmd.Name())
		if cmd != root && len(cmd.Commands()) == 0 {
			relative := strings.Join(current[1:], " ")
			if entity, ok := specs[relative]; ok {
				clioutput.WrapMutationResult(cmd, "client "+relative, entity)
				seen[relative] = true
			}
		}
		for _, child := range cmd.Commands() {
			visit(child, current)
		}
	}
	visit(root, nil)
	for path := range specs {
		if !seen[path] {
			panic(fmt.Sprintf("mutation command client %s is not registered", path))
		}
	}
}

func clientMutationFirstArg(_ *cobra.Command, args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimSpace(args[0])
}

func clientMutationFlag(name string) func(*cobra.Command, []string) string {
	return func(cmd *cobra.Command, _ []string) string {
		if value, _ := cmd.Flags().GetString(name); strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
		number, _ := cmd.Flags().GetInt(name)
		if number != 0 {
			return fmt.Sprintf("%d", number)
		}
		return ""
	}
}

func clientMutationRedirectEntity(cmd *cobra.Command, _ []string) string {
	if value, _ := cmd.Flags().GetString("domain"); strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	value, _ := cmd.Flags().GetString("cidr")
	return strings.TrimSpace(value)
}
