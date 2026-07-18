package clientcmd

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

type profileSelection struct {
	Profile   string
	Protocol  string
	Transport string
	Security  string
	Flow      string
}

func normalizeProfileSelection(value string) (profileSelection, error) {
	endpoint, err := tunnel.DefaultProfile(tunnel.Profile(strings.TrimSpace(value)))
	if err != nil {
		return profileSelection{}, fmt.Errorf("invalid profile: %w", err)
	}
	flow := ""
	if endpoint.Metadata != nil {
		flow = strings.TrimSpace(endpoint.Metadata["flow"])
	}
	return profileSelection{
		Profile:   string(endpoint.Profile),
		Protocol:  endpoint.Protocol,
		Transport: endpoint.Transport,
		Security:  endpoint.Security,
		Flow:      flow,
	}, nil
}

func validateProfileCredential(profile string, credential string) error {
	if tunnel.Profile(strings.TrimSpace(profile)) == tunnel.ProfileVLESSTLSVision {
		return tunnel.ValidateVLESSCredential(credential)
	}
	return nil
}
