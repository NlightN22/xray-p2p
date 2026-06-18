//go:build windows || linux

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type SetUserEnabledOptions struct {
	UserID  string
	All     bool
	Enabled bool
}

func SetUserEnabled(ctx context.Context, opts SetUserEnabledOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !opts.All && strings.TrimSpace(opts.UserID) == "" {
		return errUserIDRequired
	}

	configPath := pendingConfigPath()
	desired, err := loadServerDesiredConfigFromPath(configPath)
	if err != nil {
		return err
	}
	changed, err := setTrojanUsersEnabled(desired.Users, opts.UserID, opts.All, opts.Enabled)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err := saveServerTrojanUsers(configPath, desired.Users); err != nil {
		return err
	}
	return writeServerApplyRequest()
}

func setTrojanUsersEnabled(users []trojanClient, userID string, all bool, enabled bool) (bool, error) {
	if len(users) == 0 {
		return false, errors.New("no users configured")
	}
	changed := false
	trimmed := strings.TrimSpace(userID)
	for idx := range users {
		if !all && !strings.EqualFold(strings.TrimSpace(users[idx].Email), trimmed) {
			continue
		}
		disabled := !enabled
		if users[idx].Disabled == disabled {
			continue
		}
		users[idx].Disabled = disabled
		changed = true
	}
	if !all && !changed {
		for _, user := range users {
			if strings.EqualFold(strings.TrimSpace(user.Email), trimmed) {
				return false, nil
			}
		}
		return false, fmt.Errorf("user %s not found", trimmed)
	}
	return changed, nil
}
