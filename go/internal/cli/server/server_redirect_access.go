package servercmd

import (
	"fmt"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/spf13/cobra"
)

func newServerRedirectAccessCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{Use: "access", Short: "Manage redirect access policies"}
	var opts server.RedirectAccessOptions
	set := &cobra.Command{Use: "set", Short: "Replace a redirect access policy", RunE: func(cmd *cobra.Command, _ []string) error { return server.SetRedirectAccess(opts) }}
	flags := set.Flags()
	flags.StringVarP(&opts.CIDR, "cidr", "C", "", "CIDR redirect selector")
	flags.StringVarP(&opts.Domain, "domain", "d", "", "domain redirect selector")
	flags.StringVarP(&opts.Tag, "tag", "g", "", "reverse outbound tag")
	flags.StringVarP(&opts.Hostname, "host", "H", "", "reverse portal host")
	flags.StringVarP(&opts.Access, "access", "V", "", "access policy: all or restricted")
	flags.StringSliceVarP(&opts.AllowUsers, "allow-user", "U", nil, "allowed user label (repeatable)")
	flags.StringSliceVarP(&opts.AllowGroups, "allow-group", "G", nil, "allowed provider group ID (repeatable)")
	set.PreRunE = func(_ *cobra.Command, _ []string) error {
		if (opts.CIDR == "") == (opts.Domain == "") {
			return fmt.Errorf("exactly one of --cidr or --domain is required")
		}
		if opts.Tag == "" {
			return fmt.Errorf("--tag is required")
		}
		return nil
	}
	cmd.AddCommand(set)
	for _, action := range []struct{ name, short string }{{"add-user", "Add allowed users"}, {"remove-user", "Remove allowed users"}, {"add-group", "Add allowed groups"}, {"remove-group", "Remove allowed groups"}, {"clear", "Clear redirect access selectors"}} {
		action := action
		var mutation server.RedirectAccessOptions
		child := &cobra.Command{Use: action.name, Short: action.short, RunE: func(_ *cobra.Command, _ []string) error { return server.UpdateRedirectAccess(mutation, action.name) }}
		bindRedirectAccessTargetFlags(child, &mutation)
		if action.name == "add-user" || action.name == "remove-user" {
			child.Flags().StringSliceVarP(&mutation.AllowUsers, "allow-user", "U", nil, "allowed user label (repeatable)")
		}
		if action.name == "add-group" || action.name == "remove-group" {
			child.Flags().StringSliceVarP(&mutation.AllowGroups, "allow-group", "G", nil, "allowed provider group ID (repeatable)")
		}
		cmd.AddCommand(child)
	}
	return cmd
}

func bindRedirectAccessTargetFlags(cmd *cobra.Command, opts *server.RedirectAccessOptions) {
	flags := cmd.Flags()
	flags.StringVarP(&opts.CIDR, "cidr", "C", "", "CIDR redirect selector")
	flags.StringVarP(&opts.Domain, "domain", "d", "", "domain redirect selector")
	flags.StringVarP(&opts.Tag, "tag", "g", "", "reverse outbound tag")
	flags.StringVarP(&opts.Hostname, "host", "H", "", "reverse portal host")
}
