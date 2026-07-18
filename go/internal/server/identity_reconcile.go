//go:build windows || linux

package server

import (
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/identity"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
)

func reconcileAuthoritativeIdentityRemovals(doc map[string]any, state identitysync.State) (bool, error) {
	labels := removedAuthoritativeIdentityLabels(state)
	if len(labels) == 0 {
		return false, nil
	}
	desiredUsers, err := decodeServerTrojanUsers(doc)
	if err != nil {
		return false, err
	}
	changed := false
	for _, label := range labels {
		filtered := make([]trojanClient, 0, len(desiredUsers))
		removedUser := false
		for _, user := range desiredUsers {
			if strings.EqualFold(strings.TrimSpace(user.Email), label) && user.ManagedByIdentity {
				removedUser = true
				continue
			}
			filtered = append(filtered, user)
		}
		if removedUser {
			desiredUsers = filtered
			setServerUsers(doc, desiredUsers)
			changed = true
		}
		purged, err := purgeUserReverseAndRedirectsDoc(doc, RemoveUserOptions{}, label)
		if err != nil {
			return false, err
		}
		changed = changed || purged
	}
	return changed, nil
}

func removedAuthoritativeIdentityLabels(state identitysync.State) []string {
	if state.Current == nil || state.Pending == nil || state.Transaction == nil || state.Transaction.CandidateGenerationID != state.Pending.ID {
		return nil
	}
	providerSubjects := map[string]struct{}{}
	for _, id := range state.Pending.ProviderSubjects {
		providerSubjects[strings.TrimSpace(id)] = struct{}{}
	}
	hasProviderSnapshotSubjects := len(providerSubjects) > 0
	var labels []string
	for id, subject := range state.Current.Subjects {
		if _, ok := state.Pending.Subjects[id]; ok {
			continue
		}
		if hasProviderSnapshotSubjects {
			if _, ok := providerSubjects[id]; ok {
				continue
			}
		}
		label := strings.TrimSpace(subject.UserLabel)
		if identity.IsManagedUserLabel(label) {
			labels = append(labels, label)
		}
	}
	return labels
}
