//go:build windows || linux

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

var (
	errUserIDRequired   = errors.New("user identifier is required")
	errPasswordRequired = errors.New("password is required")
)

// AddUser ensures a client exists in Desired inputs.
func AddUser(ctx context.Context, opts AddUserOptions) error {
	return addUser(ctx, opts, commitServerRuntimeDoc)
}

// StageUser updates Desired only. Deploy flows apply staged changes through
// the service layer after the deployment handshake completes.
func StageUser(ctx context.Context, opts AddUserOptions) error {
	return addUser(ctx, opts, func(_ context.Context, doc map[string]any) error {
		return writeServerStateDoc(config.ConfigPath(layout.ServerConfigFileName), doc)
	})
}

func addUser(ctx context.Context, opts AddUserOptions, commit func(context.Context, map[string]any) error) error {
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
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return err
	}
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
		setServerUsers(doc, users)
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
		reverseState, err := decodeServerReverseState(doc)
		if err != nil {
			return err
		}
		if existing, ok := reverseState[channel.Tag]; ok && !strings.EqualFold(existing.UserID, channel.UserID) {
			return fmt.Errorf("reverse tag %s already assigned to %s", channel.Tag, existing.UserID)
		}
		reverseState.ensure()
		reverseState[channel.Tag] = channel
		doc[serverReverseStateKey] = reverseState
	}
	return commit(ctx, doc)
}

// RemoveUser deletes the client from Desired inputs. The operation is idempotent.
func RemoveUser(ctx context.Context, opts RemoveUserOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	userID := strings.TrimSpace(opts.UserID)
	if userID == "" {
		return errUserIDRequired
	}

	configPath := pendingConfigPath()
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return err
	}
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
		setServerUsers(doc, filtered)
	}

	purged, err := purgeUserReverseAndRedirectsDoc(doc, opts, userID)
	if err != nil {
		return err
	}
	if !removed && !purged {
		return nil
	}
	return commitServerRuntimeDoc(ctx, doc)
}

func purgeUserReverseAndRedirectsDoc(doc map[string]any, opts RemoveUserOptions, userID string) (bool, error) {
	channels := make([]serverReverseChannel, 0)
	reverseState, err := decodeServerReverseState(doc)
	if err != nil {
		return false, err
	}
	removed := make([]serverReverseChannel, 0)
	trimmedUser := strings.TrimSpace(userID)
	for tag, channel := range reverseState {
		if !strings.EqualFold(strings.TrimSpace(channel.UserID), trimmedUser) {
			continue
		}
		removed = append(removed, channel)
		delete(reverseState, tag)
	}
	if len(removed) > 0 {
		channels = append(channels, removed...)
		doc[serverReverseStateKey] = reverseState
	}

	if len(channels) == 0 {
		params, err := resolveTrojanLinkParams(pendingConfigPath(), "", strings.TrimSpace(opts.Host))
		if err == nil {
			if channel, channelErr := buildServerReverseChannel(userID, params.host); channelErr == nil {
				channels = append(channels, channel)
			}
		}
	}
	if len(channels) == 0 {
		return false, nil
	}

	redirects, err := decodeServerRedirectRules(doc)
	if err != nil {
		return false, err
	}
	changed := false
	for _, channel := range channels {
		updated, removed := redirect.RemoveRulesByTag(redirects, channel.Tag)
		if removed {
			redirects = updated
			changed = true
		}
	}
	if !changed {
		return len(removed) > 0, nil
	}
	doc[serverRedirectRulesKey] = redirects
	return true, nil
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
