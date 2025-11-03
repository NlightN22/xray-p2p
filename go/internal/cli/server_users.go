package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func runServerUser(ctx context.Context, cfg config.Config, args []string) int {
	if len(args) == 0 {
		printServerUserUsage()
		return 1
	}

	cmd := strings.ToLower(args[0])
	switch cmd {
	case "add":
		return runServerUserAdd(ctx, cfg, args[1:])
	case "remove":
		return runServerUserRemove(ctx, cfg, args[1:])
	case "list":
		return runServerUserList(ctx, cfg, args[1:])
	case "-h", "--help", "help":
		printServerUserUsage()
		return 0
	default:
		if strings.HasPrefix(cmd, "-") {
			return runServerUserList(ctx, cfg, args)
		}
		logging.Error("xp2p server user: unknown subcommand", "subcommand", args[0])
		printServerUserUsage()
		return 1
	}
}

func runServerUserAdd(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server user add", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name or absolute path")
	userID := fs.String("id", "", "Trojan client identifier")
	password := fs.String("password", "", "Trojan client password or pre-shared key")
	passwordAlias := fs.String("key", "", "Alias for --password")
	linkHost := fs.String("host", "", "public host name or IP for generated connection link")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server user add: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server user add: unexpected arguments", "args", fs.Args())
		return 2
	}

	secret := firstNonEmpty(*password, *passwordAlias)
	if strings.TrimSpace(secret) == "" {
		logging.Error("xp2p server user add: password is required")
		return 2
	}
	if strings.TrimSpace(*password) != "" && strings.TrimSpace(*passwordAlias) != "" && strings.TrimSpace(*password) != strings.TrimSpace(*passwordAlias) {
		logging.Error("xp2p server user add: conflicting password values for --password and --key")
		return 2
	}

	opts := server.AddUserOptions{
		InstallDir: firstNonEmpty(*path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Server.ConfigDir),
		UserID:     *userID,
		Password:   secret,
	}

	if err := serverUserAddFunc(ctx, opts); err != nil {
		logging.Error("xp2p server user add failed", "err", err)
		return 1
	}

	logging.Info("xp2p server user add completed", "user_id", strings.TrimSpace(*userID))

	linkOpts := server.UserLinkOptions{
		InstallDir: firstNonEmpty(*path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Server.ConfigDir),
		Host:       firstNonEmpty(*linkHost, cfg.Server.Host),
		UserID:     *userID,
	}
	if link, err := serverUserLinkFunc(ctx, linkOpts); err != nil {
		logging.Warn("xp2p server user add: unable to build trojan link", "err", err)
	} else {
		fmt.Println(link.Link)
	}
	return 0
}

func runServerUserRemove(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server user remove", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name or absolute path")
	userID := fs.String("id", "", "Trojan client identifier")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server user remove: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server user remove: unexpected arguments", "args", fs.Args())
		return 2
	}

	opts := server.RemoveUserOptions{
		InstallDir: firstNonEmpty(*path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Server.ConfigDir),
		UserID:     *userID,
	}

	if err := serverUserRemoveFunc(ctx, opts); err != nil {
		logging.Error("xp2p server user remove failed", "err", err)
		return 1
	}

	logging.Info("xp2p server user remove completed", "user_id", strings.TrimSpace(*userID))
	return 0
}

func runServerUserList(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server user list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	list := fs.Bool("list", false, "display configured Trojan users and their connection links")
	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name or absolute path")
	host := fs.String("host", "", "public host name or IP for generated connection links")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server user list: failed to parse arguments", "err", err)
		return 2
	}
	if !*list {
		logging.Error("xp2p server user list: --list flag is required")
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server user list: unexpected arguments", "args", fs.Args())
		return 2
	}

	opts := server.ListUsersOptions{
		InstallDir: firstNonEmpty(*path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(*configDir, cfg.Server.ConfigDir),
		Host:       firstNonEmpty(*host, cfg.Server.Host),
	}

	users, err := serverUserListFunc(ctx, opts)
	if err != nil {
		logging.Error("xp2p server user list failed", "err", err)
		return 1
	}

	if len(users) == 0 {
		fmt.Println("No Trojan users configured.")
		return 0
	}

	for _, user := range users {
		label := strings.TrimSpace(user.UserID)
		if label == "" {
			label = "(unnamed)"
		}
		fmt.Printf("%s: %s\n", label, user.Link)
	}
	return 0
}

func printServerUserUsage() {
	fmt.Print(`xp2p server user commands:
  add    [--path PATH] [--config-dir NAME|PATH] --id ID (--password VALUE | --key VALUE)
  remove [--path PATH] [--config-dir NAME|PATH] --id ID
  --list [--path PATH] [--config-dir NAME|PATH] [--host HOST]
`)
}
