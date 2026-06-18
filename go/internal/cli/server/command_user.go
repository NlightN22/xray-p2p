package servercmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newServerUserCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users on the server",
	}

	cmd.AddCommand(
		newServerUserAddCmd(cfg),
		newServerUserDisableCmd(cfg),
		newServerUserEnableCmd(cfg),
		newServerUserRemoveCmd(cfg),
		newServerUserListCmd(cfg),
	)
	return cmd
}

func newServerUserDisableCmd(cfg commandConfig) *cobra.Command {
	return newServerUserToggleCmd(cfg, false)
}

func newServerUserEnableCmd(cfg commandConfig) *cobra.Command {
	return newServerUserToggleCmd(cfg, true)
}

func newServerUserToggleCmd(cfg commandConfig, enabled bool) *cobra.Command {
	var opts serverUserToggleOptions
	name := "disable"
	short := "Disable a user"
	if enabled {
		name = "enable"
		short = "Enable a user"
	}
	cmd := &cobra.Command{
		Use:   name + " <id>",
		Short: short,
		Args: func(_ *cobra.Command, args []string) error {
			if opts.All {
				if len(args) > 0 {
					return fmt.Errorf("--all does not accept positional arguments")
				}
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("expected exactly one user id")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.UserID = args[0]
			}
			code := runServerUserToggle(commandContext(cmd), cfg(), opts, enabled)
			return errorForCode(code)
		},
	}
	cmd.Flags().BoolVarP(&opts.All, "all", "a", false, "enable or disable all users")
	return cmd
}

func newServerUserAddCmd(cfg commandConfig) *cobra.Command {
	var opts serverUserAddOptions
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a user and reverse portal",
		Long:  "Add a user, update inbounds.json, and create a sanitized <user><host>.rev reverse portal/routing entry so clients can mirror the bridge automatically (disable with --no-reverse).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerUserAdd(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.StringVarP(&opts.UserID, "id", "i", "", "client identifier (derives the <id><host>.rev reverse tag)")
	flags.StringVarP(&opts.Password, "password", "w", "", "client password or pre-shared key (auto-generated when omitted)")
	flags.StringVarP(&opts.Key, "key", "k", "", "alias for --password")
	flags.StringVarP(&opts.LinkHost, "host", "H", "", "public host name or IP for generated connection link")
	flags.BoolVarP(&opts.NoReverse, "no-reverse", "n", false, "skip creating reverse portal/routing entries")
	flags.BoolVarP(&opts.Force, "force", "f", false, "overwrite existing user entry")
	return cmd
}

func newServerUserRemoveCmd(cfg commandConfig) *cobra.Command {
	var opts serverUserRemoveOptions
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a user",
		Long:  "Remove a user and clean up the matching <user><host>.rev reverse portal.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerUserRemove(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.StringVarP(&opts.UserID, "id", "i", "", "client identifier")
	flags.StringVarP(&opts.Host, "host", "H", "", "public host name or IP (defaults to server host)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newServerUserListCmd(cfg commandConfig) *cobra.Command {
	var opts serverUserListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured users",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerUserList(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name or absolute path")
	flags.StringVarP(&opts.Host, "host", "H", "", "public host name or IP for generated connection links")
	flags.BoolVarP(&opts.Pending, "pending", "y", false, "list pending configuration")
	return cmd
}
