//go:build linux || windows

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestUpdateUserOnlyChangesTrojanUserFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		UserID:     "old-user",
		Password:   "old-password",
		Host:       "edge.example",
	}); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	beforeStore, err := openReverseStore(dir)
	if err != nil {
		t.Fatalf("open reverse store: %v", err)
	}
	if len(beforeStore.state) != 1 {
		t.Fatalf("expected one reverse channel, got %+v", beforeStore.state)
	}

	if err := UpdateUser(context.Background(), UpdateUserOptions{
		UserID:      "old-user",
		NewUserID:   "new-user",
		Password:    "new-password",
		NewUserSet:  true,
		PasswordSet: true,
	}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	desired, err := loadServerDesiredConfigFromPath(pendingConfigPath())
	if err != nil {
		t.Fatalf("load desired: %v", err)
	}
	if len(desired.Users) != 1 {
		t.Fatalf("expected one user, got %+v", desired.Users)
	}
	if desired.Users[0].Email != "new-user" || desired.Users[0].Password != "new-password" {
		t.Fatalf("user fields were not updated: %+v", desired.Users[0])
	}

	afterStore, err := openReverseStore(dir)
	if err != nil {
		t.Fatalf("open reverse store after update: %v", err)
	}
	for tag, channel := range beforeStore.state {
		got, ok := afterStore.state[tag]
		if !ok {
			t.Fatalf("reverse channel %s was removed", tag)
		}
		if got != channel {
			t.Fatalf("reverse channel changed: got %+v want %+v", got, channel)
		}
	}
	if _, err := os.Stat(config.ApplyRequestPath()); err != nil {
		t.Fatalf("apply request was not written: %v", err)
	}
}
