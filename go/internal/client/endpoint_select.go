package client

import (
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

var (
	ErrClientEndpointsMissing = errors.New("no client endpoints found")
	ErrClientEndpointNotFound = errors.New("client endpoint not found")
)

func ResolveMarkerTarget(installDir, host, tag string, index int) (string, int, error) {
	_ = installDir
	liveDir, err := config.LiveRoleDir("client")
	if err != nil {
		return "", 0, err
	}
	meta, err := loadLiveRuntimeMeta(liveDir)
	if err != nil {
		if strings.Contains(err.Error(), "runtime metadata missing at ") {
			return "", 0, endpointsMissingError{}
		}
		return "", 0, err
	}
	state := runtimeDesiredToClientInstallState(meta.Desired)
	if len(state.Endpoints) == 0 {
		return "", 0, endpointsMissingError{}
	}

	_, selectedIndex, err := selectEndpointByHost(state.Endpoints, host, tag, index)
	if err != nil {
		return "", 0, err
	}
	target, err := markerIPForIndex(selectedIndex)
	if err != nil {
		return "", 0, err
	}
	return target, DiagnosticsMarkerPort, nil
}

// ResolveMarkerTLSName returns the TLS name for the endpoint selected by a marker ping.
type MarkerTLS struct {
	ServerName           string
	AllowInsecure        bool
	PinnedPeerCertSHA256 string
	User                 string
	Credential           string
}

// ResolveMarkerTLS returns TLS settings for the endpoint selected by a marker ping.
func ResolveMarkerTLS(installDir, host, tag string, index int) (MarkerTLS, error) {
	_ = installDir
	liveDir, err := config.LiveRoleDir("client")
	if err != nil {
		return MarkerTLS{}, err
	}
	meta, err := loadLiveRuntimeMeta(liveDir)
	if err != nil {
		if strings.Contains(err.Error(), "runtime metadata missing at ") {
			return MarkerTLS{}, endpointsMissingError{}
		}
		return MarkerTLS{}, err
	}
	state := runtimeDesiredToClientInstallState(meta.Desired)
	selected, _, err := selectEndpointByHost(state.Endpoints, host, tag, index)
	if err != nil {
		return MarkerTLS{}, err
	}
	return MarkerTLS{
		ServerName:           strings.TrimSpace(selected.ServerName),
		AllowInsecure:        selected.AllowInsecure,
		PinnedPeerCertSHA256: strings.TrimSpace(selected.PinnedPeerCertSHA256),
		User:                 strings.TrimSpace(selected.User),
		Credential:           strings.TrimSpace(selected.Password),
	}, nil
}

func selectEndpointByHost(endpoints []clientEndpointRecord, host, tag string, index int) (clientEndpointRecord, int, error) {
	trimmedHost := strings.TrimSpace(host)
	if trimmedHost == "" {
		return clientEndpointRecord{}, -1, errors.New("host is required")
	}

	trimmedTag := strings.TrimSpace(tag)
	if trimmedTag != "" && index > 0 {
		return clientEndpointRecord{}, -1, errors.New("--endpoint and --index cannot be used together")
	}
	if index < 0 {
		return clientEndpointRecord{}, -1, fmt.Errorf("endpoint index %d is invalid", index)
	}

	matches := make([]indexedEndpoint, 0, len(endpoints))
	userMatches := make([]indexedEndpoint, 0, len(endpoints))
	for idx, ep := range endpoints {
		if strings.EqualFold(ep.Hostname, trimmedHost) {
			matches = append(matches, indexedEndpoint{index: idx, record: ep})
		}
		if strings.EqualFold(ep.User, trimmedHost) {
			userMatches = append(userMatches, indexedEndpoint{index: idx, record: ep})
		}
	}

	if trimmedTag != "" {
		for idx, ep := range endpoints {
			if strings.EqualFold(ep.Tag, trimmedTag) {
				return ep, idx, nil
			}
		}
		return clientEndpointRecord{}, -1, fmt.Errorf("outbound tag %q is not registered", trimmedTag)
	}

	if len(matches) == 0 && len(userMatches) > 0 {
		matches = userMatches
	}
	if len(matches) == 0 {
		return clientEndpointRecord{}, -1, endpointNotFoundError{host: trimmedHost}
	}

	if index > 0 {
		if index > len(matches) {
			return clientEndpointRecord{}, -1, fmt.Errorf("endpoint index %d is out of range for host %q", index, trimmedHost)
		}
		selected := matches[index-1]
		return selected.record, selected.index, nil
	}

	selected := matches[0]
	return selected.record, selected.index, nil
}

type indexedEndpoint struct {
	index  int
	record clientEndpointRecord
}

type endpointNotFoundError struct {
	host string
}

func (e endpointNotFoundError) Error() string {
	return fmt.Sprintf("client endpoint %q not found", e.host)
}

func (e endpointNotFoundError) Is(target error) bool {
	return target == ErrClientEndpointNotFound
}

type endpointsMissingError struct{}

func (e endpointsMissingError) Error() string {
	return "no client endpoints found (run xp2p client install first)"
}

func (e endpointsMissingError) Is(target error) bool {
	return target == ErrClientEndpointsMissing
}
