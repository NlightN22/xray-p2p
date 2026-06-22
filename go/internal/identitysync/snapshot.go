package identitysync

import (
	"fmt"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/identity"
)

type LabelAllocator func() (string, error)

func ApplySnapshot(current *Generation, snapshot Snapshot, now time.Time, allocate LabelAllocator) (*Generation, Status, error) {
	if !snapshot.Complete {
		return current, Status{State: SyncStatusPartial, Error: "snapshot is partial"}, nil
	}
	if err := snapshot.Provider.Validate(); err != nil {
		return current, Status{State: SyncStatusError, Error: err.Error()}, err
	}
	if allocate == nil {
		allocate = identity.NewManagedUserLabel
	}
	normalized, err := normalizeSnapshot(compatibleCurrent(current, snapshot.Provider), snapshot, now, allocate)
	if err != nil {
		return current, Status{State: SyncStatusError, Error: err.Error()}, err
	}
	return normalized, Status{
		State:       SyncStatusSuccess,
		LastSuccess: nowUTCString(now),
	}, nil
}

func normalizeSnapshot(current *Generation, snapshot Snapshot, now time.Time, allocate LabelAllocator) (*Generation, error) {
	subjectsByID := map[string]SnapshotSubject{}
	subjectDNs := map[string]string{}
	for _, subject := range snapshot.Subjects {
		id := strings.TrimSpace(subject.ExternalSubject)
		if id == "" {
			return nil, fmt.Errorf("identity subject is missing external subject")
		}
		if _, ok := subjectsByID[id]; ok {
			return nil, fmt.Errorf("duplicate identity subject %q", id)
		}
		subject.ExternalSubject = id
		subject.DisplayName = strings.TrimSpace(subject.DisplayName)
		subjectsByID[id] = subject
		if dn := normalizeDN(subject.DN); dn != "" {
			if existing, ok := subjectDNs[dn]; ok && existing != id {
				return nil, fmt.Errorf("ambiguous identity DN %q", subject.DN)
			}
			subjectDNs[dn] = id
		}
	}

	groupsByID := map[string]SnapshotGroup{}
	groupDNs := map[string]string{}
	for _, group := range snapshot.Groups {
		id := strings.TrimSpace(group.ID)
		if id == "" {
			return nil, fmt.Errorf("identity group is missing id")
		}
		if _, ok := groupsByID[id]; ok {
			return nil, fmt.Errorf("duplicate identity group %q", id)
		}
		group.ID = id
		group.DisplayName = strings.TrimSpace(group.DisplayName)
		groupsByID[id] = group
		if dn := normalizeDN(group.DN); dn != "" {
			if existing, ok := groupDNs[dn]; ok && existing != id {
				return nil, fmt.Errorf("ambiguous group DN %q", group.DN)
			}
			groupDNs[dn] = id
		}
	}

	resolvedGroups := map[string]Group{}
	directSubjectGroups := map[string][]string{}
	for id, group := range groupsByID {
		resolved, err := resolveGroupMembers(group, subjectsByID, groupsByID, subjectDNs, groupDNs)
		if err != nil {
			return nil, err
		}
		resolvedGroups[id] = resolved
		for _, subjectID := range resolved.DirectMembers {
			directSubjectGroups[subjectID] = append(directSubjectGroups[subjectID], id)
		}
	}
	if err := rejectCycles(resolvedGroups); err != nil {
		return nil, err
	}

	reachableGroups, reachableSubjects := scopeReachability(snapshot.Provider.Scope, resolvedGroups)
	if len(snapshot.Provider.Scope) == 0 {
		for id := range resolvedGroups {
			reachableGroups[id] = true
		}
		for id := range subjectsByID {
			reachableSubjects[id] = true
		}
	}

	next := &Generation{
		ID:                 generationID(snapshot, now),
		ProviderInstanceID: snapshot.Provider.InstanceID,
		CreatedAt:          nowUTCString(now),
		Subjects:           map[string]Subject{},
		Groups:             map[string]Group{},
	}
	for id, group := range resolvedGroups {
		if !reachableGroups[id] {
			continue
		}
		group.DirectMembers = filterReachable(group.DirectMembers, reachableSubjects)
		group.DirectGroups = filterReachable(group.DirectGroups, reachableGroups)
		next.Groups[id] = group
	}
	for id, raw := range subjectsByID {
		if !reachableSubjects[id] {
			continue
		}
		label, provisioned, err := labelForSubject(current, id, allocate)
		if err != nil {
			return nil, err
		}
		groups := dedupeSorted(directSubjectGroups[id])
		next.Subjects[id] = Subject{
			ExternalSubject: id,
			UserLabel:       label,
			DisplayName:     raw.DisplayName,
			Active:          true,
			Provisioned:     provisioned,
			DirectGroups:    groups,
		}
	}
	return next, nil
}

func resolveGroupMembers(group SnapshotGroup, subjects map[string]SnapshotSubject, groups map[string]SnapshotGroup, subjectDNs, groupDNs map[string]string) (Group, error) {
	memberSubjects := append([]string{}, group.MemberSubjects...)
	memberGroups := append([]string{}, group.MemberGroups...)
	for _, dn := range group.MemberDNs {
		normalized := normalizeDN(dn)
		if normalized == "" {
			return Group{}, fmt.Errorf("group %q contains malformed member DN", group.ID)
		}
		if id, ok := subjectDNs[normalized]; ok {
			memberSubjects = append(memberSubjects, id)
			continue
		}
		if id, ok := groupDNs[normalized]; ok {
			memberGroups = append(memberGroups, id)
			continue
		}
		return Group{}, fmt.Errorf("group %q contains unresolved member DN %q", group.ID, dn)
	}
	for _, id := range memberSubjects {
		if _, ok := subjects[strings.TrimSpace(id)]; !ok {
			return Group{}, fmt.Errorf("group %q references unknown subject %q", group.ID, id)
		}
	}
	for _, id := range memberGroups {
		if _, ok := groups[strings.TrimSpace(id)]; !ok {
			return Group{}, fmt.Errorf("group %q references unknown nested group %q", group.ID, id)
		}
	}
	return Group{
		ID:            group.ID,
		DisplayName:   group.DisplayName,
		DirectMembers: dedupeSorted(memberSubjects),
		DirectGroups:  dedupeSorted(memberGroups),
	}, nil
}
