package ui

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	envPollSeconds = "XP2P_UI_POLL_SECONDS"
	envAutoStart   = "XP2P_UI_AUTOSTART"
)

type Settings struct {
	PollInterval time.Duration
	AutoStart    bool
}

func LoadSettings() Settings {
	return Settings{
		PollInterval: pollIntervalFromEnv(),
		AutoStart:    autoStartFromEnv(),
	}
}

func pollIntervalFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv(envPollSeconds))
	if raw == "" {
		return 5 * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 5 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func autoStartFromEnv() bool {
	raw := strings.TrimSpace(os.Getenv(envAutoStart))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
