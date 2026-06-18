//go:build linux || windows

package client

import (
	"sort"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// ReverseListOptions controls client reverse enumeration.
type ReverseListOptions struct {
	InstallDir string
	ConfigDir  string
	Pending    bool
}

// ReverseRecord describes a client reverse tunnel.
type ReverseRecord struct {
	Tag         string
	Host        string
	User        string
	Domain      string
	EndpointTag string
	Bridge      bool
	DirectRule  bool
	Disabled    bool
}

// ListReverse enumerates client reverse tunnels from Desired inputs.
func ListReverse(opts ReverseListOptions) ([]ReverseRecord, error) {
	statePath := config.ConfigPath(layout.ClientConfigFileName)
	state, err := loadClientInstallState(statePath)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(state.Reverse))
	for tag := range state.Reverse {
		keys = append(keys, tag)
	}
	sort.Strings(keys)

	records := make([]ReverseRecord, 0, len(keys))
	for _, key := range keys {
		channel := state.Reverse[key]
		records = append(records, ReverseRecord{
			Tag:         channel.Tag,
			Host:        channel.Host,
			User:        channel.UserID,
			Domain:      channel.Domain,
			EndpointTag: channel.EndpointTag,
			Bridge:      true,
			DirectRule:  true,
			Disabled:    channel.Disabled,
		})
	}
	_ = opts
	return records, nil
}
