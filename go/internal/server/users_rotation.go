//go:build windows || linux

package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

const defaultRotationTTL = 24 * time.Hour

type RotateUserOptions struct {
	UserID string
	TTL    time.Duration
}

type RotateUserResult struct {
	UserID             string    `json:"user_id"`
	Credential         string    `json:"credential"`
	PreviousValidUntil time.Time `json:"previous_valid_until"`
	Generation         int       `json:"generation"`
}

// RotateUser replaces only the active protocol-neutral credential. The runtime
// candidate contains the new credential, so the old one immediately stops
// authenticating tunnel traffic after a successful apply.
func RotateUser(ctx context.Context, opts RotateUserOptions) (RotateUserResult, error) {
	label := strings.TrimSpace(opts.UserID)
	if label == "" {
		return RotateUserResult{}, errUserIDRequired
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = defaultRotationTTL
	}
	doc, err := loadServerStateDoc(pendingConfigPath())
	if err != nil {
		return RotateUserResult{}, err
	}
	desired, err := loadServerDesiredConfigFromPath(pendingConfigPath())
	if err != nil {
		return RotateUserResult{}, err
	}
	for i := range desired.Users {
		if !strings.EqualFold(desired.Users[i].Email, label) {
			continue
		}
		if desired.Users[i].Disabled {
			return RotateUserResult{}, fmt.Errorf("user %s is disabled", label)
		}
		if err := rotateUserCredential(&desired.Users[i], ttl); err != nil {
			return RotateUserResult{}, err
		}
		setServerUsers(doc, desired.Users)
		if err := commitServerRuntimeDoc(ctx, doc); err != nil {
			return RotateUserResult{}, err
		}
		return RotateUserResult{
			UserID:             desired.Users[i].Email,
			Credential:         desired.Users[i].Password,
			PreviousValidUntil: desired.Users[i].RotationExpiresAt.UTC(),
			Generation:         desired.Users[i].CredentialGeneration,
		}, nil
	}
	return RotateUserResult{}, fmt.Errorf("user %s not found", label)
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
