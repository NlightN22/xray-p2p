package servercmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

func bindServerMutationResults(root *cobra.Command) {
	first := serverMutationFirstArg
	redirect := serverMutationRedirectEntity
	specs := map[string]func(*cobra.Command, []string) string{
		"forward add":                  serverMutationFlag("target"),
		"forward remove":               serverMutationFlag("listen-port"),
		"ha channel create":            first,
		"ha channel disable":           first,
		"ha channel finalize":          first,
		"ha channel rebind":            first,
		"ha channel rebind-endpoint":   first,
		"ha group create":              first,
		"ha group remove":              serverMutationConstant("server ha group"),
		"ha group update":              serverMutationConstant("server ha group"),
		"ha member add":                first,
		"ha member remove":             first,
		"ha member reprioritize":       first,
		"ha peer add":                  first,
		"ha peer remove":               first,
		"ha peer self":                 first,
		"ha redirect add":              redirect,
		"ha redirect remove":           redirect,
		"ha sync":                      serverMutationConstant("server ha"),
		"identity detach":              serverMutationConstant("server identity"),
		"identity select":              first,
		"identity sync":                serverMutationConstant("server identity"),
		"redirect access add-group":    redirect,
		"redirect access add-user":     redirect,
		"redirect access clear":        redirect,
		"redirect access remove-group": redirect,
		"redirect access remove-user":  redirect,
		"redirect access set":          redirect,
		"redirect add":                 redirect,
		"redirect disable":             redirect,
		"redirect enable":              redirect,
		"redirect remove":              redirect,
		"reverse disable":              first,
		"reverse enable":               first,
		"user disable":                 first,
		"user enable":                  first,
		"user remove":                  serverMutationFlag("id"),
		"user update":                  first,
	}
	bindServerMutationSpecs(root, specs)
}

func bindServerMutationSpecs(
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
				clioutput.WrapMutationResult(cmd, "server "+relative, entity)
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
			panic(fmt.Sprintf("mutation command server %s is not registered", path))
		}
	}
}

func serverMutationFirstArg(_ *cobra.Command, args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimSpace(args[0])
}

func serverMutationFlag(name string) func(*cobra.Command, []string) string {
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

func serverMutationRedirectEntity(cmd *cobra.Command, _ []string) string {
	if value, _ := cmd.Flags().GetString("domain"); strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	value, _ := cmd.Flags().GetString("cidr")
	return strings.TrimSpace(value)
}

func serverMutationConstant(value string) func(*cobra.Command, []string) string {
	return func(*cobra.Command, []string) string { return value }
}
