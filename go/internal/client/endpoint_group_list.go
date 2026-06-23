//go:build linux || windows

package client

import (
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

type EndpointGroupRecord struct {
	GroupID       string
	Tag           string
	Members       []string
	Mode          string
	ActiveTag     string
	CooldownUntil string
	Revision      uint64
}

func ListEndpointGroups() ([]EndpointGroupRecord, error) {
	desired, err := loadClientInstallState(config.ConfigPath(layout.ClientConfigFileName))
	if err != nil {
		return nil, err
	}
	if err := desired.validateEndpointGroups(); err != nil {
		return nil, err
	}
	liveDir, err := config.LiveRoleDir("client")
	if err != nil {
		return nil, err
	}
	selector, err := loadEndpointSelectorState(filepath.Join(liveDir, layout.ClientEndpointSelectorStateFileName))
	if err != nil {
		return nil, err
	}
	records := make([]EndpointGroupRecord, 0, len(desired.EndpointGroups))
	for _, group := range desired.EndpointGroups {
		state := selector.Groups[lowerGroupID(group.GroupID)]
		records = append(records, EndpointGroupRecord{GroupID: group.GroupID, Tag: group.Tag, Members: append([]string(nil), group.Members...), Mode: string(group.Mode), ActiveTag: state.ActiveTag, CooldownUntil: state.CooldownUntil.UTC().Format("2006-01-02T15:04:05Z"), Revision: selector.Revision})
	}
	return records, nil
}

func lowerGroupID(id string) string { return strings.ToLower(strings.TrimSpace(id)) }
