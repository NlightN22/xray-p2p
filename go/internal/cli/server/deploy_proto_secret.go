package servercmd

import (
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/identity"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func generateSecret(size int) (string, error) {
	return identity.NewSecret(size)
}

func generateDeployPassword(profile string) (string, error) {
	if tunnel.Profile(strings.TrimSpace(profile)) == tunnel.ProfileVLESSTLSVision {
		return tunnel.NewCredential()
	}
	return generateSecret(18)
}

func validateDeployProfileCredential(profile string, credential string) error {
	if tunnel.Profile(strings.TrimSpace(profile)) == tunnel.ProfileVLESSTLSVision {
		return tunnel.ValidateVLESSCredential(credential)
	}
	return nil
}
