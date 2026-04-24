package clientcmd

import (
	"fmt"
	"strings"
)

type targetClientMode struct {
	set        bool
	tunEnabled bool
	tunMode    string
	tunModeSet bool
}

func parseTargetClientMode(value string) (targetClientMode, error) {
	raw := strings.ToLower(strings.TrimSpace(value))
	if raw == "" {
		return targetClientMode{}, nil
	}
	switch raw {
	case "proxy":
		return targetClientMode{set: true, tunEnabled: false}, nil
	case "tun":
		return targetClientMode{set: true, tunEnabled: true}, nil
	}

	if strings.HasPrefix(raw, "tun:") {
		mode := strings.TrimSpace(strings.TrimPrefix(raw, "tun:"))
		switch mode {
		case "split", "full":
			return targetClientMode{
				set:        true,
				tunEnabled: true,
				tunMode:    mode,
				tunModeSet: true,
			}, nil
		default:
			return targetClientMode{}, fmt.Errorf("invalid mode %q (use proxy, tun, tun:split, or tun:full)", value)
		}
	}

	return targetClientMode{}, fmt.Errorf("invalid mode %q (use proxy or tun)", value)
}

func normalizeTunModeFlag(name string, value string, set bool) (string, bool, error) {
	if !set {
		return "", false, nil
	}
	mode := strings.ToLower(strings.TrimSpace(value))
	switch mode {
	case "split", "full":
		return mode, true, nil
	default:
		return "", false, fmt.Errorf("%s must be split or full", name)
	}
}

