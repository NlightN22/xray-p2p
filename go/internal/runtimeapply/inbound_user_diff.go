package runtimeapply

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

func classifyInboundUsersDiff(current, candidate []byte) (Diff, bool, error) {
	currentDoc, err := parseJSONDocument(current)
	if err != nil {
		return Diff{}, false, fmt.Errorf("parse current xray config: %w", err)
	}
	candidateDoc, err := parseJSONDocument(candidate)
	if err != nil {
		return Diff{}, false, fmt.Errorf("parse candidate xray config: %w", err)
	}
	currentInbounds, ok, err := detachTopLevelObjectArray(currentDoc, "inbounds")
	if err != nil {
		return Diff{}, false, err
	}
	if !ok {
		return Diff{}, false, nil
	}
	candidateInbounds, ok, err := detachTopLevelObjectArray(candidateDoc, "inbounds")
	if err != nil {
		return Diff{}, false, err
	}
	if !ok {
		return Diff{}, false, nil
	}
	if !reflect.DeepEqual(currentDoc, candidateDoc) {
		return Diff{}, false, nil
	}
	diff, ok, err := classifyInboundUsers(currentInbounds, candidateInbounds)
	return diff, ok, err
}

func classifyInboundUsers(currentInbounds, candidateInbounds []map[string]any) (Diff, bool, error) {
	currentByTag, err := objectsByTag(currentInbounds, "tag", "inbound")
	if err != nil {
		return unsupported(err.Error()), true, nil
	}
	candidateByTag, err := objectsByTag(candidateInbounds, "tag", "inbound")
	if err != nil {
		return unsupported(err.Error()), true, nil
	}
	if !sameStringKeys(currentByTag, candidateByTag) {
		return Diff{}, false, nil
	}

	diff := Diff{Kind: DiffInboundUsers}
	changed := false
	for tag, current := range currentByTag {
		candidate := candidateByTag[tag]
		currentUsers, currentOK, err := detachTrojanClients(current)
		if err != nil {
			return unsupported(err.Error()), true, nil
		}
		if !currentOK {
			if !reflect.DeepEqual(current, candidate) {
				return Diff{}, false, nil
			}
			continue
		}
		candidateUsers, candidateOK, err := detachTrojanClients(candidate)
		if err != nil {
			return unsupported(err.Error()), true, nil
		}
		if !candidateOK || !reflect.DeepEqual(current, candidate) {
			return Diff{}, false, nil
		}
		userDiff, err := classifyTrojanUsers(tag, currentUsers, candidateUsers)
		if err != nil {
			return unsupported(err.Error()), true, nil
		}
		if len(userDiff.AddedInboundUsers) > 0 || len(userDiff.RemovedInboundUsers) > 0 {
			changed = true
			diff.AddedInboundUsers = append(diff.AddedInboundUsers, userDiff.AddedInboundUsers...)
			diff.RemovedInboundUsers = append(diff.RemovedInboundUsers, userDiff.RemovedInboundUsers...)
		}
	}
	if !changed {
		return Diff{Kind: DiffNoop}, true, nil
	}
	sort.Slice(diff.AddedInboundUsers, func(i, j int) bool {
		return userChangeLess(diff.AddedInboundUsers[i], diff.AddedInboundUsers[j])
	})
	sort.Slice(diff.RemovedInboundUsers, func(i, j int) bool {
		return userChangeLess(diff.RemovedInboundUsers[i], diff.RemovedInboundUsers[j])
	})
	return diff, true, nil
}

func detachTrojanClients(inbound map[string]any) ([]map[string]any, bool, error) {
	protocol, _ := inbound["protocol"].(string)
	if !strings.EqualFold(strings.TrimSpace(protocol), "trojan") {
		return nil, false, nil
	}
	settings, ok := inbound["settings"].(map[string]any)
	if !ok {
		return nil, false, errors.New("trojan inbound settings are missing")
	}
	rawClients, ok := settings["clients"].([]any)
	if !ok {
		return nil, false, errors.New("trojan inbound clients are missing")
	}
	clients := make([]map[string]any, 0, len(rawClients))
	for _, raw := range rawClients {
		client, ok := raw.(map[string]any)
		if !ok {
			return nil, false, errors.New("trojan client is not an object")
		}
		clients = append(clients, client)
	}
	settings["clients"] = []any{}
	return clients, true, nil
}

func classifyTrojanUsers(tag string, currentUsers, candidateUsers []map[string]any) (Diff, error) {
	currentByEmail, err := usersByEmail(currentUsers)
	if err != nil {
		return Diff{}, err
	}
	candidateByEmail, err := usersByEmail(candidateUsers)
	if err != nil {
		return Diff{}, err
	}
	diff := Diff{Kind: DiffInboundUsers}
	for email, current := range currentByEmail {
		candidate, exists := candidateByEmail[email]
		if !exists {
			diff.RemovedInboundUsers = append(diff.RemovedInboundUsers, InboundUserChange{
				InboundTag: tag,
				Email:      email,
				Password:   stringField(current, "password"),
				User:       current,
			})
			continue
		}
		if !reflect.DeepEqual(current, candidate) {
			return Diff{}, fmt.Errorf("trojan user %s changed in place", email)
		}
	}
	for email, candidate := range candidateByEmail {
		if _, exists := currentByEmail[email]; exists {
			continue
		}
		diff.AddedInboundUsers = append(diff.AddedInboundUsers, InboundUserChange{
			InboundTag: tag,
			Email:      email,
			Password:   stringField(candidate, "password"),
			User:       candidate,
		})
	}
	return diff, nil
}

func usersByEmail(users []map[string]any) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any, len(users))
	for _, user := range users {
		email := stringField(user, "email")
		if email == "" {
			return nil, errors.New("trojan user without email")
		}
		if stringField(user, "password") == "" {
			return nil, fmt.Errorf("trojan user %s without password", email)
		}
		key := strings.ToLower(email)
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate trojan user %s", email)
		}
		result[key] = user
	}
	return result, nil
}

func sameStringKeys(left, right map[string]map[string]any) bool {
	if len(left) != len(right) {
		return false
	}
	for key := range left {
		if _, ok := right[key]; !ok {
			return false
		}
	}
	return true
}

func stringField(item map[string]any, field string) string {
	value, _ := item[field].(string)
	return strings.TrimSpace(value)
}

func userChangeLess(left, right InboundUserChange) bool {
	if left.InboundTag != right.InboundTag {
		return left.InboundTag < right.InboundTag
	}
	return left.Email < right.Email
}
