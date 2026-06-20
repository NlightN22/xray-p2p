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

// RotateUser replaces only the active protocol-neutral credential. The runtime
// candidate contains the new credential, so the old one immediately stops
// authenticating tunnel traffic after a successful apply.
func RotateUser(ctx context.Context, opts RotateUserOptions) error {
	label := strings.TrimSpace(opts.UserID)
	if label == "" {
		return errUserIDRequired
	}
	credential, err := tunnel.NewCredential()
	if err != nil {
		return err
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
		desired.Users[i].PreviousCredentialForRotation = desired.Users[i].Password
		desired.Users[i].Password = credential
		desired.Users[i].RotationExpiresAt = time.Now().UTC().Add(ttl)
		desired.Users[i].CredentialGeneration++
		if desired.Users[i].CredentialGeneration == 0 {
			desired.Users[i].CredentialGeneration = 1
		}
		setServerUsers(doc, desired.Users)
		return commitServerRuntimeDoc(ctx, doc)
	}
	return fmt.Errorf("user %s not found", label)
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
