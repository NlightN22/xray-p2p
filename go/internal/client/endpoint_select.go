package client

import (
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

var (
	ErrClientEndpointsMissing = errors.New("xp2p: no client endpoints found")
	ErrClientEndpointNotFound = errors.New("xp2p: client endpoint not found")
)

func ResolveMarkerTarget(installDir, host, tag string, index int) (string, int, error) {
	_ = installDir
	liveDir, err := config.LiveRoleDir("client")
	if err != nil {
		return "", 0, err
	}
	meta, err := loadLiveRuntimeMeta(liveDir)
	if err != nil {
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

func selectEndpointByHost(endpoints []clientEndpointRecord, host, tag string, index int) (clientEndpointRecord, int, error) {
	trimmedHost := strings.TrimSpace(host)
	if trimmedHost == "" {
		return clientEndpointRecord{}, -1, errors.New("xp2p: host is required")
	}

	trimmedTag := strings.TrimSpace(tag)
	if trimmedTag != "" && index > 0 {
		return clientEndpointRecord{}, -1, errors.New("xp2p: --endpoint and --index cannot be used together")
	}
	if index < 0 {
		return clientEndpointRecord{}, -1, fmt.Errorf("xp2p: endpoint index %d is invalid", index)
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
		return clientEndpointRecord{}, -1, fmt.Errorf("xp2p: outbound tag %q is not registered", trimmedTag)
	}

	if len(matches) == 0 && len(userMatches) > 0 {
		matches = userMatches
	}
	if len(matches) == 0 {
		return clientEndpointRecord{}, -1, endpointNotFoundError{host: trimmedHost}
	}

	if index > 0 {
		if index > len(matches) {
			return clientEndpointRecord{}, -1, fmt.Errorf("xp2p: endpoint index %d is out of range for host %q", index, trimmedHost)
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
	return fmt.Sprintf("xp2p: client endpoint %q not found", e.host)
}

func (e endpointNotFoundError) Is(target error) bool {
	return target == ErrClientEndpointNotFound
}

type endpointsMissingError struct{}

func (e endpointsMissingError) Error() string {
	return "xp2p: no client endpoints found (run xp2p client install first)"
}

func (e endpointsMissingError) Is(target error) bool {
	return target == ErrClientEndpointsMissing
}
