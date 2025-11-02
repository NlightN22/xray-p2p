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
	case "-h", "--help", "help":
		printServerUserUsage()
		return 0
	default:
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

func printServerUserUsage() {
	fmt.Print(`xp2p server user commands:
  add    [--path PATH] [--config-dir NAME|PATH] --id ID (--password VALUE | --key VALUE)
  remove [--path PATH] [--config-dir NAME|PATH] --id ID
`)
}
