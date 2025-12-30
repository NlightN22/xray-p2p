package client

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/installstate"
)

var (
	ErrClientEndpointsMissing = errors.New("xp2p: no client endpoints found")
	ErrClientEndpointNotFound = errors.New("xp2p: client endpoint not found")
)

func ResolveMarkerTarget(installDir, host, tag string, index int) (string, error) {
	installDir = strings.TrimSpace(installDir)
	if installDir == "" {
		return "", errors.New("xp2p: client install dir is required to resolve endpoints")
	}
	statePath := filepath.Join(installDir, installstate.FileNameForKind(installstate.KindClient))
	state, err := loadClientInstallState(statePath)
	if err != nil {
		return "", err
	}
	if len(state.Endpoints) == 0 {
		return "", endpointsMissingError{}
	}

	_, selectedIndex, err := selectEndpointByHost(state.Endpoints, host, tag, index)
	if err != nil {
		return "", err
	}
	return markerIPForIndex(selectedIndex)
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
	for idx, ep := range endpoints {
		if strings.EqualFold(ep.Hostname, trimmedHost) {
			matches = append(matches, indexedEndpoint{index: idx, record: ep})
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
