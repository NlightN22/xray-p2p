package servercmd

import (
	"context"
	"fmt"
	"strings"

	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
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
	NoReverse bool
	Force     bool
}

type serverUserRemoveOptions struct {
	Path      string
	ConfigDir string
	UserID    string
	Host      string
}

type serverUserListOptions struct {
	Path      string
	ConfigDir string
	Host      string
	Pending   bool
}

func runServerUserAdd(ctx context.Context, cfg config.Config, opts serverUserAddOptions) int {
	passwordValue := strings.TrimSpace(opts.Password)
	keyValue := strings.TrimSpace(opts.Key)
	if passwordValue != "" && keyValue != "" && passwordValue != keyValue {
		logging.Error("xp2p server user add: conflicting values for --password and --key")
		return 2
	}
	secret := firstNonEmpty(passwordValue, keyValue)
	generated := false
	if strings.TrimSpace(secret) == "" {
		secretValue, err := generateRandomSecret(18)
		if err != nil {
			logging.Error("xp2p server user add: generate password failed", "err", err)
			return 1
		}
		secret = secretValue
		generated = true
	}
	if err := clishared.ValidateRFC3986Unreserved(secret); err != nil {
		logging.Error("xp2p server user add: invalid password", "err", err)
		return 2
	}

	host := firstNonEmpty(opts.LinkHost, cfg.Server.Host)
	if strings.TrimSpace(host) == "" {
		installDir := firstNonEmpty(opts.Path, cfg.Server.InstallDir)
		configDirName := firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir)
		configDirPath, err := resolveConfigDirPath(installDir, configDirName)
		if err != nil {
			logging.Error("xp2p server user add: resolve config dir failed", "err", err)
			return 1
		}
		candidates, err := server.ResolveLinkHostCandidates(configDirPath, "")
		if err != nil {
			logging.Error("xp2p server user add: resolve host failed", "err", err)
			return 1
		}
		if len(candidates) == 1 {
			host = candidates[0]
		} else {
			selected, err := promptChoiceFunc("Select host for reverse portal/link generation:", candidates)
			if err != nil {
				logging.Error("xp2p server user add: host selection failed", "err", err)
				return 1
			}
			host = selected
		}
	}

	addOpts := server.AddUserOptions{
		InstallDir: firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		UserID:     opts.UserID,
		Password:   secret,
		Host:       host,
		NoReverse:  opts.NoReverse,
		Force:      opts.Force,
	}

	if err := serverUserAddFunc(ctx, addOpts); err != nil {
		logging.Error("xp2p server user add failed", "err", err)
		return 1
	}

	if generated {
		logging.Info("xp2p server user add: generated password", "user_id", strings.TrimSpace(opts.UserID), "password", secret)
	}
	logging.Info("xp2p server user add completed", "user_id", strings.TrimSpace(opts.UserID))

	if strings.TrimSpace(host) != "" {
		linkOpts := server.UserLinkOptions{
			InstallDir: addOpts.InstallDir,
			ConfigDir:  addOpts.ConfigDir,
			Host:       host,
			UserID:     opts.UserID,
			Pending:    true,
		}
		if link, err := serverUserLinkFunc(ctx, linkOpts); err != nil {
			logging.Warn("xp2p server user add: unable to build trojan link", "err", err)
		} else {
			fmt.Println(link.Link)
		}
	}
	return 0
}

func runServerUserRemove(ctx context.Context, cfg config.Config, opts serverUserRemoveOptions) int {
	host := firstNonEmpty(opts.Host, cfg.Server.Host)

	removeOpts := server.RemoveUserOptions{
		InstallDir: firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		UserID:     opts.UserID,
		Host:       host,
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
		Pending:    opts.Pending,
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
