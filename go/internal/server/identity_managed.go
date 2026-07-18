//go:build windows || linux

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identity"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
)

func ProvisionIdentity(ctx context.Context, opts ProvisionIdentityOptions) (UserLink, error) {
	var link UserLink
	err := identitysync.DefaultOperationLock().With(ctx, func() error {
		var provisionErr error
		link, provisionErr = provisionIdentityLocked(ctx, opts)
		return provisionErr
	})
	return link, err
}

func provisionIdentityLocked(ctx context.Context, opts ProvisionIdentityOptions) (UserLink, error) {
	if err := ctx.Err(); err != nil {
		return UserLink{}, err
	}
	label := strings.TrimSpace(opts.UserLabel)
	if label == "" {
		return UserLink{}, errUserIDRequired
	}
	if !identity.IsManagedUserLabel(label) {
		return UserLink{}, errors.New("identity provisioning requires a managed idp- user label")
	}

	store := identitysync.DefaultStore()
	state, err := store.Load()
	if err != nil {
		return UserLink{}, err
	}
	if state.Current == nil || state.Current.Detached {
		return UserLink{}, errors.New("identity current generation is not available")
	}
	subjectID := ""
	for id, subject := range state.Current.Subjects {
		if strings.EqualFold(subject.UserLabel, label) {
			subjectID = id
			break
		}
	}
	if subjectID == "" {
		return UserLink{}, fmt.Errorf("identity label %s not found", label)
	}

	configPath := pendingConfigPath()
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return UserLink{}, err
	}
	previousDoc, err := cloneServerStateDoc(doc)
	if err != nil {
		return UserLink{}, err
	}
	desired, err := loadServerDesiredConfigFromPath(configPath)
	if err != nil {
		return UserLink{}, err
	}

	password, err := identity.NewTunnelCredential()
	if err != nil {
		return UserLink{}, err
	}
	users := desired.Users
	found := false
	for i := range users {
		if !strings.EqualFold(users[i].Email, label) {
			continue
		}
		users[i].Password = password
		users[i].ManagedByIdentity = true
		users[i].Disabled = false
		found = true
		break
	}
	if !found {
		users = append(users, trojanClient{Email: label, Password: password, ManagedByIdentity: true})
	}
	setServerUsers(doc, users)

	params, err := resolveTrojanLinkParams(configPath, "", strings.TrimSpace(opts.Host))
	if err != nil {
		return UserLink{}, err
	}
	channel, err := buildServerReverseChannel(label, params.host)
	if err != nil {
		return UserLink{}, err
	}
	reverseState, err := decodeServerReverseState(doc)
	if err != nil {
		return UserLink{}, err
	}
	if existing, ok := reverseState[channel.Tag]; ok && !strings.EqualFold(existing.UserID, channel.UserID) {
		return UserLink{}, fmt.Errorf("reverse tag %s already assigned to %s", channel.Tag, existing.UserID)
	}
	reverseState.ensure()
	reverseState[channel.Tag] = channel
	doc[serverReverseStateKey] = reverseState

	if err := commitServerRuntimeDoc(ctx, doc); err != nil {
		return UserLink{}, err
	}
	if err := store.SetProvisionedByLabel(label, true); err != nil {
		if rollbackErr := commitServerRuntimeDoc(ctx, previousDoc); rollbackErr != nil {
			return UserLink{}, fmt.Errorf("mark identity provisioned: %w; rollback failed: %v", err, rollbackErr)
		}
		return UserLink{}, err
	}
	return GetUserLink(ctx, UserLinkOptions{
		InstallDir: config.ConfigRoot(),
		ConfigDir:  opts.ConfigDir,
		Host:       opts.Host,
		UserID:     label,
		Pending:    true,
	})
}

func cloneServerStateDoc(doc map[string]any) (map[string]any, error) {
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var clone map[string]any
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return clone, nil
}

func clearManagedIdentityProvisioned(label string) error {
	if !identity.IsManagedUserLabel(label) {
		return nil
	}
	return identitysync.DefaultStore().SetProvisionedByLabel(label, false)
}

func removeManagedIdentityByLabel(label string) error {
	if !identity.IsManagedUserLabel(label) {
		return nil
	}
	return identitysync.DefaultStore().RemoveSubjectByLabel(label)
}

func RemoveAuthoritativeIdentity(ctx context.Context, label string) error {
	return identitysync.DefaultOperationLock().With(ctx, func() error {
		return removeAuthoritativeIdentityLocked(ctx, label)
	})
}

func removeAuthoritativeIdentityLocked(ctx context.Context, label string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return errUserIDRequired
	}
	if !identity.IsManagedUserLabel(label) {
		return errors.New("authoritative identity removal requires a managed idp- user label")
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
	filtered := make([]trojanClient, 0, len(desired.Users))
	removedUser := false
	for _, user := range desired.Users {
		if strings.EqualFold(user.Email, label) && user.ManagedByIdentity {
			removedUser = true
			continue
		}
		filtered = append(filtered, user)
	}
	if removedUser {
		setServerUsers(doc, filtered)
	}
	purged, err := purgeUserReverseAndRedirectsDoc(doc, RemoveUserOptions{}, label)
	if err != nil {
		return err
	}
	if removedUser || purged {
		if err := commitServerRuntimeDoc(ctx, doc); err != nil {
			return err
		}
	}
	return removeManagedIdentityByLabel(label)
}
