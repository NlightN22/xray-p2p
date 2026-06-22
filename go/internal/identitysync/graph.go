package identitysync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/identity"
)

func rejectCycles(groups map[string]Group) error {
	const maxDepth = 16
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var walk func(id string, depth int) error
	walk = func(id string, depth int) error {
		if depth > maxDepth {
			return fmt.Errorf("identity group nesting exceeds depth %d", maxDepth)
		}
		if visiting[id] {
			return fmt.Errorf("identity group cycle includes %q", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, child := range groups[id].DirectGroups {
			if err := walk(child, depth+1); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for id := range groups {
		if err := walk(id, 1); err != nil {
			return err
		}
	}
	return nil
}

func scopeReachability(scope []string, groups map[string]Group) (map[string]bool, map[string]bool) {
	reachableGroups := map[string]bool{}
	reachableSubjects := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		if reachableGroups[id] {
			return
		}
		group, ok := groups[id]
		if !ok {
			return
		}
		reachableGroups[id] = true
		for _, subjectID := range group.DirectMembers {
			reachableSubjects[subjectID] = true
		}
		for _, groupID := range group.DirectGroups {
			walk(groupID)
		}
	}
	for _, id := range scope {
		walk(strings.TrimSpace(id))
	}
	return reachableGroups, reachableSubjects
}

func labelForSubject(current *Generation, id string, allocate LabelAllocator) (string, bool, error) {
	if current != nil {
		if existing, ok := current.Subjects[id]; ok && existing.UserLabel != "" {
			return existing.UserLabel, existing.Provisioned, nil
		}
	}
	label, err := allocate()
	if err != nil {
		return "", false, fmt.Errorf("allocate managed label: %w", err)
	}
	if !identity.IsManagedUserLabel(label) {
		return "", false, fmt.Errorf("allocated label %q is not managed", label)
	}
	return label, false, nil
}

func compatibleCurrent(current *Generation, provider ProviderRef) *Generation {
	if current == nil || current.Detached {
		return nil
	}
	if current.ProviderInstanceID == "" {
		return current
	}
	if current.ProviderInstanceID != provider.InstanceID {
		return nil
	}
	return current
}

func generationID(snapshot Snapshot, now time.Time) string {
	h := sha256.New()
	h.Write([]byte(snapshot.Provider.InstanceID))
	h.Write([]byte(string(snapshot.Provider.Kind)))
	h.Write([]byte(nowUTCString(now)))
	for _, id := range snapshotSubjectIDs(snapshot) {
		h.Write([]byte(id))
	}
	for _, id := range snapshotGroupIDs(snapshot) {
		h.Write([]byte(id))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func snapshotSubjectIDs(snapshot Snapshot) []string {
	ids := make([]string, 0, len(snapshot.Subjects))
	for _, subject := range snapshot.Subjects {
		ids = append(ids, strings.TrimSpace(subject.ExternalSubject))
	}
	sort.Strings(ids)
	return ids
}

func snapshotGroupIDs(snapshot Snapshot) []string {
	ids := make([]string, 0, len(snapshot.Groups))
	for _, group := range snapshot.Groups {
		ids = append(ids, strings.TrimSpace(group.ID))
	}
	sort.Strings(ids)
	return ids
}

func filterReachable(values []string, reachable map[string]bool) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if reachable[value] {
			filtered = append(filtered, value)
		}
	}
	return dedupeSorted(filtered)
}

func dedupeSorted(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeDN(dn string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(dn)), " "))
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

func nowUTCString(t time.Time) string {
	if t.IsZero() {
		t = nowUTC()
	}
	return t.UTC().Format(time.RFC3339)
}
