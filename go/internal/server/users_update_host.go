//go:build windows || linux

package server

import (
	"context"
	"fmt"
	"strings"
)

// UpdateUser updates only the selected Trojan user email/password fields.
func UpdateUser(ctx context.Context, opts UpdateUserOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	userID := strings.TrimSpace(opts.UserID)
	if userID == "" {
		return errUserIDRequired
	}
	if !opts.NewUserSet && !opts.PasswordSet {
		return fmt.Errorf("at least one of new user id or password is required")
	}
	newUserID := strings.TrimSpace(opts.NewUserID)
	if opts.NewUserSet && newUserID == "" {
		return errUserIDRequired
	}
	password := strings.TrimSpace(opts.Password)
	if opts.PasswordSet && password == "" {
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

	found := -1
	for idx := range desired.Users {
		if strings.EqualFold(strings.TrimSpace(desired.Users[idx].Email), userID) {
			found = idx
			continue
		}
		if opts.NewUserSet && strings.EqualFold(strings.TrimSpace(desired.Users[idx].Email), newUserID) {
			return fmt.Errorf("user %s already exists", newUserID)
		}
	}
	if found < 0 {
		return fmt.Errorf("user %s not found", userID)
	}

	if opts.NewUserSet {
		desired.Users[found].Email = newUserID
	}
	if opts.PasswordSet {
		desired.Users[found].Password = password
	}
	setServerUsers(doc, desired.Users)
	return commitServerRuntimeDoc(ctx, doc)
}
