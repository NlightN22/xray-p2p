package servercmd

import (
	"context"
	"fmt"
	"strings"

	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identity"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

type serverUserAddOptions struct {
	Path      string
	ConfigDir string
	UserID    string
	Password  string
	Key       string
	LinkHost  string
	Link      string
	NoReverse bool
	Force     bool
}

type serverUserRemoveOptions struct {
	Path      string
	ConfigDir string
	UserID    string
	Host      string
}

type serverUserUpdateOptions struct {
	Path        string
	ConfigDir   string
	UserID      string
	NewUserID   string
	Password    string
	NewUserSet  bool
	PasswordSet bool
}

type serverUserListOptions struct {
	Path      string
	ConfigDir string
	Host      string
	Pending   bool
}

type serverUserToggleOptions struct {
	UserID string
	All    bool
}

type serverUserCredentialResult struct {
	UserID   string   `json:"user_id"`
	Password string   `json:"password"`
	Link     *string  `json:"link"`
	Warnings []string `json:"warnings"`
}

type serverUserListResult struct {
	Users []serverUserResult `json:"users"`
}

type serverUserResult struct {
	UserID   string `json:"user_id"`
	Disabled bool   `json:"disabled"`
	Link     string `json:"link"`
}

func runServerUserAdd(ctx context.Context, cfg config.Config, opts serverUserAddOptions) int {
	linkValue := strings.TrimSpace(opts.Link)
	if linkValue != "" {
		if strings.TrimSpace(opts.UserID) != "" || strings.TrimSpace(opts.Password) != "" || strings.TrimSpace(opts.Key) != "" {
			logging.Error("xp2p server user add: --link cannot be combined with --id, --password, or --key")
			return 2
		}
		parsed, err := tunnel.ParseLink(linkValue)
		if err != nil {
			logging.Error("xp2p server user add: invalid --link", "err", err)
			return 2
		}
		if !strings.EqualFold(parsed.Endpoint.Protocol, "trojan") {
			logging.Error("xp2p server user add: invalid --link", "err", "connection link protocol is not trojan")
			return 2
		}
		opts.UserID = parsed.User.UserLabel
		opts.Password = tunnel.ActiveCredential(parsed.User)
		if strings.TrimSpace(opts.LinkHost) == "" {
			opts.LinkHost = parsed.Endpoint.Host
		}
	}

	passwordValue := strings.TrimSpace(opts.Password)
	keyValue := strings.TrimSpace(opts.Key)
	if passwordValue != "" && keyValue != "" && passwordValue != keyValue {
		logging.Error("xp2p server user add: conflicting values for --password and --key")
		return 2
	}
	secret := firstNonEmpty(passwordValue, keyValue)
	generated := false
	if strings.TrimSpace(secret) == "" {
		secretValue, err := identity.NewTunnelCredential()
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
			if clioutput.EnabledContext(ctx) {
				logging.Error("xp2p server user add: --host is required when multiple link hosts are available")
				return 2
			}
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

	var linkValueResult *string
	warnings := make([]string, 0)
	if strings.TrimSpace(host) != "" {
		linkOpts := server.UserLinkOptions{
			InstallDir: addOpts.InstallDir,
			ConfigDir:  addOpts.ConfigDir,
			Host:       host,
			UserID:     opts.UserID,
			Pending:    true,
		}
		if link, err := serverUserLinkFunc(ctx, linkOpts); err != nil {
			warning := "unable to build connection link"
			warnings = append(warnings, warning)
			if !clioutput.EnabledContext(ctx) {
				logging.Warn("xp2p server user add: "+warning, "err", err)
			}
		} else {
			linkValueResult = &link.Link
			if !clioutput.EnabledContext(ctx) {
				fmt.Println(link.Link)
			}
		}
	}
	if clioutput.EnabledContext(ctx) {
		if err := clioutput.SetResultContext(ctx, serverUserCredentialResult{
			UserID:   strings.TrimSpace(opts.UserID),
			Password: secret,
			Link:     linkValueResult,
			Warnings: warnings,
		}); err != nil {
			logging.Error("xp2p server user add: publish JSON result failed", "err", err)
			return 1
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

func runServerUserUpdate(ctx context.Context, cfg config.Config, opts serverUserUpdateOptions) int {
	if !opts.NewUserSet && !opts.PasswordSet {
		logging.Error("xp2p server user update: at least one of --new-id or --password is required")
		return 2
	}
	if opts.NewUserSet && strings.TrimSpace(opts.NewUserID) == "" {
		logging.Error("xp2p server user update: --new-id must not be empty")
		return 2
	}
	if opts.PasswordSet {
		if strings.TrimSpace(opts.Password) == "" {
			logging.Error("xp2p server user update: --password must not be empty")
			return 2
		}
		if err := clishared.ValidateRFC3986Unreserved(strings.TrimSpace(opts.Password)); err != nil {
			logging.Error("xp2p server user update: invalid password", "err", err)
			return 2
		}
	}

	updateOpts := server.UpdateUserOptions{
		InstallDir:  firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:   firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		UserID:      opts.UserID,
		NewUserID:   opts.NewUserID,
		Password:    opts.Password,
		NewUserSet:  opts.NewUserSet,
		PasswordSet: opts.PasswordSet,
	}
	if err := serverUserUpdateFunc(ctx, updateOpts); err != nil {
		logging.Error("xp2p server user update failed", "err", err)
		return 1
	}
	logging.Info("xp2p server user update completed", "user_id", strings.TrimSpace(opts.UserID))
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
	if clioutput.EnabledContext(ctx) {
		result := serverUserListResult{Users: make([]serverUserResult, 0, len(users))}
		for _, user := range users {
			result.Users = append(result.Users, serverUserResult{
				UserID:   strings.TrimSpace(user.UserID),
				Disabled: user.Disabled,
				Link:     user.Link,
			})
		}
		if err := clioutput.SetResultContext(ctx, result); err != nil {
			logging.Error("xp2p server user list: publish JSON result failed", "err", err)
			return 1
		}
		return 0
	}

	if len(users) == 0 {
		fmt.Println("No users configured.")
		return 0
	}

	for _, user := range users {
		label := strings.TrimSpace(user.UserID)
		if label == "" {
			label = "(unnamed)"
		}
		if user.Disabled {
			label += " [disabled]"
		}
		fmt.Printf("%s: %s\n", label, user.Link)
	}
	return 0
}

func runServerUserToggle(ctx context.Context, _ config.Config, opts serverUserToggleOptions, enabled bool) int {
	if err := server.SetUserEnabled(ctx, server.SetUserEnabledOptions{
		UserID:  opts.UserID,
		All:     opts.All,
		Enabled: enabled,
	}); err != nil {
		action := "disable"
		if enabled {
			action = "enable"
		}
		logging.Error("xp2p server user "+action+" failed", "err", err)
		return 1
	}
	action := "disabled"
	if enabled {
		action = "enabled"
	}
	if opts.All {
		logging.Info("xp2p server users " + action)
	} else {
		logging.Info("xp2p server user "+action, "user_id", strings.TrimSpace(opts.UserID))
	}
	return 0
}
