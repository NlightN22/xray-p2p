package root

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

type runtimeAPIState struct {
	rules     map[string]bool
	inbounds  map[string]bool
	outbounds map[string]bool
	users     map[string]map[string]bool
}

func (s runtimeAPIState) clone() runtimeAPIState {
	result := newRuntimeAPIState()
	copyBoolMap(result.rules, s.rules)
	copyBoolMap(result.inbounds, s.inbounds)
	copyBoolMap(result.outbounds, s.outbounds)
	for tag, users := range s.users {
		result.users[tag] = make(map[string]bool, len(users))
		copyBoolMap(result.users[tag], users)
	}
	return result
}

func newRuntimeAPIState() runtimeAPIState {
	return runtimeAPIState{
		rules:     make(map[string]bool),
		inbounds:  make(map[string]bool),
		outbounds: make(map[string]bool),
		users:     make(map[string]map[string]bool),
	}
}

func parseRuntimeAPIState(t *testing.T, data []byte) runtimeAPIState {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	state := newRuntimeAPIState()
	if routing, _ := doc["routing"].(map[string]any); routing != nil {
		for _, rule := range objectList(routing["rules"]) {
			state.rules[stringField(rule, "ruleTag")] = true
		}
	}
	for _, inbound := range objectList(doc["inbounds"]) {
		tag := stringField(inbound, "tag")
		state.inbounds[tag] = true
		settings, _ := inbound["settings"].(map[string]any)
		for _, client := range objectList(settings["clients"]) {
			email := strings.ToLower(stringField(client, "email"))
			if email == "" {
				continue
			}
			if state.users[tag] == nil {
				state.users[tag] = make(map[string]bool)
			}
			state.users[tag][email] = true
		}
	}
	for _, outbound := range objectList(doc["outbounds"]) {
		state.outbounds[stringField(outbound, "tag")] = true
	}
	return state
}

func objectList(value any) []map[string]any {
	raw, _ := value.([]any)
	result := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if object, ok := item.(map[string]any); ok {
			result = append(result, object)
		}
	}
	return result
}

func stringField(value map[string]any, name string) string {
	result, _ := value[name].(string)
	return result
}

func copyBoolMap(dst, src map[string]bool) {
	for key, value := range src {
		dst[key] = value
	}
}

func sortedKeys[T any](values map[string]T) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func normalizedRuntimeMeta(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	removeVolatileRuntimeFields(result)
	return result
}

func removeVolatileRuntimeFields(value any) {
	switch current := value.(type) {
	case map[string]any:
		for _, key := range []string{"compiled_at", "issued_at", "valid_until"} {
			delete(current, key)
		}
		for _, child := range current {
			removeVolatileRuntimeFields(child)
		}
	case []any:
		for _, child := range current {
			removeVolatileRuntimeFields(child)
		}
	}
}

type roleMutationState struct {
	desired []byte
	live    map[string][]byte
	request []byte
}

func snapshotRoleMutationState(t *testing.T, role string) roleMutationState {
	t.Helper()
	desired, err := os.ReadFile(roleDesiredPath(role))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	liveDir, err := config.LiveRoleDir(role)
	if err != nil {
		t.Fatal(err)
	}
	request, err := os.ReadFile(config.ApplyRequestPath())
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return roleMutationState{
		desired: desired,
		live:    snapshotDirectory(t, liveDir),
		request: request,
	}
}

func assertRuntimeFailureState(t *testing.T, before, after roleMutationState) {
	t.Helper()
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("runtime failure changed Desired/Live/apply.request:\nbefore=%#v\nafter=%#v", before, after)
	}
	errorData, err := os.ReadFile(config.ApplyErrorPath())
	if err != nil {
		t.Fatalf("runtime failure did not persist apply.error: %v", err)
	}
	if len(errorData) == 0 {
		t.Fatal("runtime failure persisted an empty apply.error")
	}
}

func roleDesiredPath(role string) string {
	name := layout.ServerConfigFileName
	if role == apply.RoleClient {
		name = layout.ClientConfigFileName
	}
	return config.ConfigPath(name)
}

func snapshotDirectory(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.Clean(relative)] = data
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return files
}
