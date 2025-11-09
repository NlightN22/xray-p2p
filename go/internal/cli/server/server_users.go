package servercmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

type serverUserAddOptions struct {
	Path      string
	ConfigDir string
	UserID    string
	Password  string
	Key       string
	LinkHost  string
}

type serverUserRemoveOptions struct {
	Path      string
	ConfigDir string
	UserID    string
}

type serverUserListOptions struct {
	Path      string
	ConfigDir string
	Host      string
}

func runServerUserAdd(ctx context.Context, cfg config.Config, opts serverUserAddOptions) int {
	secret := firstNonEmpty(opts.Password, opts.Key)
	if strings.TrimSpace(secret) == "" {
		logging.Error("xp2p server user add: --password (or --key) is required")
		return 2
	}
	if strings.TrimSpace(opts.Password) != "" && strings.TrimSpace(opts.Key) != "" && strings.TrimSpace(opts.Password) != strings.TrimSpace(opts.Key) {
		logging.Error("xp2p server user add: conflicting values for --password and --key")
		return 2
	}

	addOpts := server.AddUserOptions{
		InstallDir: firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		UserID:     opts.UserID,
		Password:   secret,
	}

	if err := serverUserAddFunc(ctx, addOpts); err != nil {
		logging.Error("xp2p server user add failed", "err", err)
		return 1
	}

	logging.Info("xp2p server user add completed", "user_id", strings.TrimSpace(opts.UserID))

	linkOpts := server.UserLinkOptions{
		InstallDir: firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		Host:       firstNonEmpty(opts.LinkHost, cfg.Server.Host),
		UserID:     opts.UserID,
	}
	if link, err := serverUserLinkFunc(ctx, linkOpts); err != nil {
		logging.Warn("xp2p server user add: unable to build trojan link", "err", err)
	} else {
		fmt.Println(link.Link)
	}
	return 0
}

func runServerUserRemove(ctx context.Context, cfg config.Config, opts serverUserRemoveOptions) int {
	removeOpts := server.RemoveUserOptions{
		InstallDir: firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		UserID:     opts.UserID,
	}

	if err := serverUserRemoveFunc(ctx, removeOpts); err != nil {
		logging.Error("xp2p server user remove failed", "err", err)
		return 1
	}

	logging.Info("xp2p server user remove completed", "user_id", strings.TrimSpace(opts.UserID))
	return 0
}

func runServerUserList(ctx context.Context, cfg config.Config, opts serverUserListOptions) int {
	listOpts := server.ListUsersOptions{
		InstallDir: firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		Host:       firstNonEmpty(opts.Host, cfg.Server.Host),
	}

	users, err := serverUserListFunc(ctx, listOpts)
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
