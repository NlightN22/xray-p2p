package server

import (
	"errors"
	"sort"
	"strings"
)

var (
	ErrServerReverseMissing      = errors.New("xp2p: no server reverse channels found")
	ErrServerReverseNotFound     = errors.New("xp2p: server reverse channel not found")
	ErrServerReverseNotSpecified = errors.New("xp2p: reverse user or tag is required")
	ErrServerReverseAmbiguous    = errors.New("xp2p: reverse user matches multiple channels")
)

func ResolveServerMarkerTarget(installDir, userOrTag string, index int) (string, int, error) {
	if index > 0 {
		return "", 0, errors.New("xp2p: server reverse selection does not support --index")
	}
	stateDoc, err := loadServerStateDoc(serverStatePath(installDir))
	if err != nil {
		return "", 0, err
	}
	reverseState, err := decodeServerReverseState(stateDoc)
	if err != nil {
		return "", 0, err
	}
	if len(reverseState) == 0 {
		return "", 0, ErrServerReverseMissing
	}

	_, selectedIndex, err := selectReverseChannel(reverseState, userOrTag)
	if err != nil {
		return "", 0, err
	}
	target, err := markerIPForIndex(selectedIndex)
	if err != nil {
		return "", 0, err
	}
	return target, DiagnosticsMarkerPort, nil
}

func selectReverseChannel(state serverReverseState, userOrTag string) (serverReverseChannel, int, error) {
	if len(state) == 0 {
		return serverReverseChannel{}, -1, ErrServerReverseMissing
	}

	tags := sortedReverseTags(state)
	tagIndex := make(map[string]int, len(tags))
	for idx, tag := range tags {
		tagIndex[strings.ToLower(tag)] = idx
	}

	trimmed := strings.TrimSpace(userOrTag)
	if trimmed == "" {
		return serverReverseChannel{}, -1, ErrServerReverseNotSpecified
	}

	if idx, ok := tagIndex[strings.ToLower(trimmed)]; ok {
		tag := tags[idx]
		return state[tag], idx, nil
	}

	matches := make([]serverReverseChannel, 0, len(state))
	for _, tag := range tags {
		channel := state[tag]
		if strings.EqualFold(channel.UserID, trimmed) {
			matches = append(matches, channel)
		}
	}
	if len(matches) == 0 {
		return serverReverseChannel{}, -1, ErrServerReverseNotFound
	}
	if len(matches) > 1 {
		return serverReverseChannel{}, -1, ErrServerReverseAmbiguous
	}
	match := matches[0]
	idx := tagIndex[strings.ToLower(match.Tag)]
	return match, idx, nil
}

func sortedReverseTags(state serverReverseState) []string {
	if len(state) == 0 {
		return []string{}
	}
	tags := make([]string, 0, len(state))
	for tag := range state {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}
