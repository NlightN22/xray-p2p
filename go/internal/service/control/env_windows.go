//go:build windows

package control

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const serviceEnvValueName = "Environment"

func SetServiceEnv(ctx context.Context, role Role, vars map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(vars) == 0 {
		return nil
	}

	name, err := serviceName(role)
	if err != nil {
		return err
	}
	path := fmt.Sprintf(`SYSTEM\CurrentControlSet\Services\%s`, name)
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open service registry key %s: %w", path, err)
	}
	defer key.Close()

	existing, _, err := key.GetStringsValue(serviceEnvValueName)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("read service environment: %w", err)
	}
	updated := mergeServiceEnv(existing, vars)
	if err := key.SetStringsValue(serviceEnvValueName, updated); err != nil {
		return fmt.Errorf("set service environment: %w", err)
	}
	return nil
}

func mergeServiceEnv(existing []string, vars map[string]string) []string {
	remaining := make(map[string]string, len(vars))
	for key, val := range vars {
		remaining[key] = val
	}

	updated := make([]string, 0, len(existing)+len(vars))
	for _, entry := range existing {
		key, _, ok := splitEnvEntry(entry)
		if !ok {
			updated = append(updated, entry)
			continue
		}
		if val, ok := remaining[key]; ok {
			updated = append(updated, key+"="+val)
			delete(remaining, key)
			continue
		}
		updated = append(updated, entry)
	}

	if len(remaining) == 0 {
		return updated
	}

	keys := make([]string, 0, len(remaining))
	for key := range remaining {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		updated = append(updated, key+"="+remaining[key])
	}
	return updated
}

func splitEnvEntry(entry string) (string, string, bool) {
	idx := strings.Index(entry, "=")
	if idx <= 0 {
		return "", "", false
	}
	return entry[:idx], entry[idx+1:], true
}
