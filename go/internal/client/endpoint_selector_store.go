package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

func loadEndpointSelectorState(path string) (endpointSelectorState, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return endpointSelectorState{Groups: map[string]endpointGroupSelector{}}, nil
	}
	if err != nil {
		return endpointSelectorState{}, fmt.Errorf("read endpoint selector state: %w", err)
	}
	state := endpointSelectorState{}
	if err := json.Unmarshal(data, &state); err != nil {
		return endpointSelectorState{}, fmt.Errorf("decode endpoint selector state: %w", err)
	}
	if state.Groups == nil {
		state.Groups = map[string]endpointGroupSelector{}
	}
	return state, nil
}

func saveEndpointSelectorState(path string, state endpointSelectorState) error {
	if state.Groups == nil {
		state.Groups = map[string]endpointGroupSelector{}
	}
	state.Revision++
	return configio.WriteJSON(path, state, configio.WriteOptions{})
}

func selectorJournalPath(statePath string) string {
	return filepath.Join(filepath.Dir(statePath), "endpoint-selector.journal.json")
}

func commitEndpointSelectorState(statePath string, state endpointSelectorState) error {
	if err := saveEndpointSelectorState(statePath, state); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(selectorJournalPath(statePath), append(data, '\n'), 0o644)
}
