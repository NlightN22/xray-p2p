//go:build windows || linux

package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

const defaultRotationTTL = 24 * time.Hour

type RotateUserOptions struct {
	UserID string
	TTL    time.Duration
}

// ForceRotateLegacyCredentials replaces every non-UUID active credential by
// the standard rotation path. It is safe to call on every service start.
func ForceRotateLegacyCredentials(ctx context.Context) error {
	doc, err := loadServerStateDoc(pendingConfigPath())
	if err != nil {
		return err
	}
	desired, err := loadServerDesiredConfigFromPath(pendingConfigPath())
	if err != nil {
		return err
	}
	changed := false
	for index := range desired.Users {
		if desired.Users[index].Disabled || tunnel.IsUUIDCredential(desired.Users[index].Password) {
			continue
		}
		if err := rotateUserCredential(&desired.Users[index], defaultRotationTTL); err != nil {
			return err
		}
		changed = true
		logging.Info("forced credential rotation staged", "user_label", desired.Users[index].Email)
	}
	if !changed {
		return nil
	}
	setServerUsers(doc, desired.Users)
	return commitServerRuntimeDoc(ctx, doc)
}

// RotateUser replaces only the active protocol-neutral credential. The runtime
// candidate contains the new credential, so the old one immediately stops
// authenticating tunnel traffic after a successful apply.
func RotateUser(ctx context.Context, opts RotateUserOptions) error {
	label := strings.TrimSpace(opts.UserID)
	if label == "" {
		return errUserIDRequired
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = defaultRotationTTL
	}
	doc, err := loadServerStateDoc(pendingConfigPath())
	if err != nil {
		return err
	}
	desired, err := loadServerDesiredConfigFromPath(pendingConfigPath())
	if err != nil {
		return err
	}
	for i := range desired.Users {
		if !strings.EqualFold(desired.Users[i].Email, label) {
			continue
		}
		if desired.Users[i].Disabled {
			return fmt.Errorf("user %s is disabled", label)
		}
		if err := rotateUserCredential(&desired.Users[i], ttl); err != nil {
			return err
		}
		setServerUsers(doc, desired.Users)
		return commitServerRuntimeDoc(ctx, doc)
	}
	return fmt.Errorf("user %s not found", label)
}

func rotateUserCredential(user *trojanClient, ttl time.Duration) error {
	previous := user.Password
	credential, err := tunnel.NewCredential()
	if err != nil {
		return err
	}
	user.Password = credential
	setRotationWindow(user, previous, ttl)
	return nil
}

func setRotationWindow(user *trojanClient, previous string, ttl time.Duration) {
	if previous != "" {
		user.PreviousCredentialForRotation = previous
	} else {
		user.PreviousCredentialForRotation = ""
	}
	user.RotationExpiresAt = time.Now().UTC().Add(ttl)
	user.CredentialGeneration++
	if user.CredentialGeneration == 0 {
		user.CredentialGeneration = 1
	}
}

// AcknowledgeCredential closes a matching rotation window through the same
// runtime-first state publication path as the rotation itself.
func AcknowledgeCredential(ctx context.Context, label string, generation int) error {
	doc, err := loadServerStateDoc(pendingConfigPath())
	if err != nil {
		return err
	}
	desired, err := loadServerDesiredConfigFromPath(pendingConfigPath())
	if err != nil {
		return err
	}
	for i := range desired.Users {
		if !strings.EqualFold(desired.Users[i].Email, strings.TrimSpace(label)) {
			continue
		}
		if desired.Users[i].CredentialGeneration != generation || desired.Users[i].PreviousCredentialForRotation == "" {
			return fmt.Errorf("rotation acknowledgement is invalid")
		}
		desired.Users[i].PreviousCredentialForRotation = ""
		desired.Users[i].RotationExpiresAt = time.Time{}
		setServerUsers(doc, desired.Users)
		return commitServerRuntimeDoc(ctx, doc)
	}
	return fmt.Errorf("rotation acknowledgement is invalid")
}
