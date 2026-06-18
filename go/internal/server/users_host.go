//go:build windows || linux

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	errUserIDRequired   = errors.New("user identifier is required")
	errPasswordRequired = errors.New("password is required")
)

// AddUser ensures a Trojan client exists in Desired inputs.
func AddUser(ctx context.Context, opts AddUserOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	userID := strings.TrimSpace(opts.UserID)
	if userID == "" {
		return errUserIDRequired
	}
	password := strings.TrimSpace(opts.Password)
	if password == "" {
		return errPasswordRequired
	}

	configPath := pendingConfigPath()
	desired, err := loadServerDesiredConfigFromPath(configPath)
	if err != nil {
		return err
	}

	users := desired.Users
	updated := false
	found := false
	for idx := range users {
		if !strings.EqualFold(strings.TrimSpace(users[idx].Email), userID) {
			continue
		}
		found = true
		if !opts.Force {
			return fmt.Errorf("user %s already exists (use --force to update)", userID)
		}
		if users[idx].Password != password {
			users[idx].Password = password
			updated = true
		}
		break
	}
	if !found {
		users = append(users, trojanClient{Email: userID, Password: password})
		updated = true
	}
	if updated {
		if err := saveServerTrojanUsers(configPath, users); err != nil {
			return err
		}
	}

	if !opts.NoReverse {
		params, err := resolveTrojanLinkParams(configPath, "", strings.TrimSpace(opts.Host))
		if err != nil {
			return err
		}
		channel, err := buildServerReverseChannel(userID, params.host)
		if err != nil {
			return err
		}
		store, err := openReverseStore(opts.InstallDir)
		if err != nil {
			return err
		}
		if err := store.ensureAvailable(channel); err != nil {
			return err
		}
		if err := applyServerReverseChannel(&store, opts.InstallDir, opts.ConfigDir, channel); err != nil {
			return err
		}
	}
	return writeServerApplyRequest()
}

// RemoveUser deletes the Trojan client from Desired inputs. The operation is idempotent.
func RemoveUser(ctx context.Context, opts RemoveUserOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	userID := strings.TrimSpace(opts.UserID)
	if userID == "" {
		return errUserIDRequired
	}

	configPath := pendingConfigPath()
	desired, err := loadServerDesiredConfigFromPath(configPath)
	if err != nil {
		return err
	}

	users := desired.Users
	filtered := make([]trojanClient, 0, len(users))
	removed := false
	for _, user := range users {
		if strings.EqualFold(strings.TrimSpace(user.Email), userID) {
			removed = true
			continue
		}
		filtered = append(filtered, user)
	}
	if removed {
		if err := saveServerTrojanUsers(configPath, filtered); err != nil {
			return err
		}
	}

	params, err := resolveTrojanLinkParams(configPath, "", strings.TrimSpace(opts.Host))
	if err == nil {
		if channel, channelErr := buildServerReverseChannel(userID, params.host); channelErr == nil {
			if store, storeErr := openReverseStore(opts.InstallDir); storeErr == nil {
				_ = purgeServerReverseChannel(&store, opts.InstallDir, opts.ConfigDir, channel)
			}
		}
	}
	return writeServerApplyRequest()
}

func ListUsers(ctx context.Context, opts ListUsersOptions) ([]UserLink, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	configPath := pendingConfigPath()
	desired, err := loadServerDesiredConfigFromPath(configPath)
	if err != nil {
		return nil, err
	}

	result := make([]UserLink, 0, len(desired.Users))
	for _, user := range desired.Users {
		link, err := GetUserLink(ctx, UserLinkOptions{
			InstallDir: opts.InstallDir,
			ConfigDir:  opts.ConfigDir,
			Host:       opts.Host,
			UserID:     user.Email,
			Pending:    true,
		})
		if err != nil {
			result = append(result, UserLink{
				UserID:   user.Email,
				Password: user.Password,
				Disabled: user.Disabled,
			})
			continue
		}
		if link.Password == "" {
			link.Password = user.Password
		}
		link.Disabled = user.Disabled
		result = append(result, link)
	}
	return result, nil
}

func GetUserLink(ctx context.Context, opts UserLinkOptions) (UserLink, error) {
	if err := ctx.Err(); err != nil {
		return UserLink{}, err
	}

	userID := strings.TrimSpace(opts.UserID)
	if userID == "" {
		return UserLink{}, errUserIDRequired
	}

	configPath := pendingConfigPath()
	desired, err := loadServerDesiredConfigFromPath(configPath)
	if err != nil {
		return UserLink{}, err
	}

	var client trojanClient
	found := false
	for _, entry := range desired.Users {
		if strings.EqualFold(strings.TrimSpace(entry.Email), userID) {
			client = entry
			found = true
			break
		}
	}
	if !found {
		return UserLink{}, fmt.Errorf("user %s not found", userID)
	}

	params, err := resolveTrojanLinkParams(configPath, "", strings.TrimSpace(opts.Host))
	if err != nil {
		return UserLink{}, err
	}

	link, err := buildTrojanLink(params.host, params.port, client.Password, client.Email, params.tlsEnabled, params.pinnedPeerSHA256, params.verifyPeerCertName)
	if err != nil {
		return UserLink{}, err
	}

	return UserLink{
		UserID:   client.Email,
		Password: client.Password,
		Link:     link,
		Disabled: client.Disabled,
	}, nil
}
