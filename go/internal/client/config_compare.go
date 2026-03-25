package client

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
)

func jsonConfigEqual(path string, doc any) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	var existing any
	if err := json.Unmarshal(data, &existing); err != nil {
		return false, nil
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}
	var incoming any
	if err := json.Unmarshal(raw, &incoming); err != nil {
		return false, err
	}
	return reflect.DeepEqual(existing, incoming), nil
}
