package ui

import (
	"testing"
	"time"
)

func TestLoadSettingsDefaults(t *testing.T) {
	t.Setenv(envPollSeconds, "")
	t.Setenv(envAutoStart, "")

	settings := LoadSettings()
	if settings.PollInterval != 5*time.Second {
		t.Fatalf("unexpected poll interval: %v", settings.PollInterval)
	}
	if !settings.AutoStart {
		t.Fatal("expected autostart enabled by default")
	}
}

func TestLoadSettingsOverrides(t *testing.T) {
	t.Setenv(envPollSeconds, "2")
	t.Setenv(envAutoStart, "0")

	settings := LoadSettings()
	if settings.PollInterval != 2*time.Second {
		t.Fatalf("unexpected poll interval: %v", settings.PollInterval)
	}
	if settings.AutoStart {
		t.Fatal("expected autostart disabled")
	}
}
