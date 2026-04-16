//go:build linux || windows

package server

import (
	"sort"
)

// ReverseListOptions controls server reverse enumeration.
type ReverseListOptions struct {
	InstallDir string
	ConfigDir  string
	Pending    bool
}

// ReverseRecord describes a server reverse tunnel.
type ReverseRecord struct {
	Domain      string
	Host        string
	User        string
	Tag         string
	Portal      bool
	RoutingRule bool
}

// ListReverse enumerates server reverse tunnels from Desired inputs.
func ListReverse(opts ReverseListOptions) ([]ReverseRecord, error) {
	statePath := pendingConfigPath()
	stateDoc, err := loadServerStateDoc(statePath)
	if err != nil {
		return nil, err
	}
	reverseState, err := decodeServerReverseState(stateDoc)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(reverseState))
	for tag := range reverseState {
		keys = append(keys, tag)
	}
	sort.Strings(keys)

	records := make([]ReverseRecord, 0, len(keys))
	for _, tag := range keys {
		channel := reverseState[tag]
		records = append(records, ReverseRecord{
			Domain:      channel.Domain,
			Host:        channel.Host,
			User:        channel.UserID,
			Tag:         channel.Tag,
			Portal:      true,
			RoutingRule: true,
		})
	}
	_ = opts
	return records, nil
}
