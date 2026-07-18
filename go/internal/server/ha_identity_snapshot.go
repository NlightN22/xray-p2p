package server

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/ha"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
)

func attachHAIdentitySnapshot(generation *ha.Generation) error {
	state, err := identitysync.DefaultStore().Load()
	if err != nil {
		return err
	}
	if state.Current == nil || state.Current.Detached {
		return nil
	}
	identityPayload, err := json.Marshal(state.Current)
	if err != nil {
		return err
	}
	provisionedPayload, err := json.Marshal(provisionedIdentityLabels(*state.Current))
	if err != nil {
		return err
	}
	generation.IdentityACL = identityPayload
	generation.Provisioned = provisionedPayload
	return nil
}

func provisionedIdentityLabels(generation identitysync.Generation) []string {
	labels := make([]string, 0, len(generation.Subjects))
	for _, subject := range generation.Subjects {
		label := strings.TrimSpace(subject.UserLabel)
		if subject.Provisioned && label != "" {
			labels = append(labels, label)
		}
	}
	sort.Slice(labels, func(i, j int) bool { return strings.ToLower(labels[i]) < strings.ToLower(labels[j]) })
	return labels
}
