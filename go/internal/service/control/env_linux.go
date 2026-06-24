//go:build linux

package control

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func SetServiceEnv(ctx context.Context, role Role, vars map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(vars) == 0 {
		return nil
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return ErrUnsupported
	}
	unit := unitName(role)
	if unit == "" {
		return fmt.Errorf("unsupported role %q", role)
	}
	dir := filepath.Join("/etc/systemd/system", unit+".d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create service environment override dir: %w", err)
	}
	data, err := systemdEnvironmentDropIn(vars)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "10-xp2p-env.conf"), data, 0o644); err != nil {
		return fmt.Errorf("write service environment override: %w", err)
	}
	return runSystemctl(ctx, "daemon-reload")
}

func systemdEnvironmentDropIn(vars map[string]string) ([]byte, error) {
	keys := make([]string, 0, len(vars))
	for key := range vars {
		if !validEnvironmentKey(key) {
			return nil, fmt.Errorf("invalid environment key %q", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("[Service]\n")
	for _, key := range keys {
		b.WriteString("Environment=")
		b.WriteString(strconv.Quote(key + "=" + vars[key]))
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

func validEnvironmentKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if i == 0 && r >= '0' && r <= '9' {
			return false
		}
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}
