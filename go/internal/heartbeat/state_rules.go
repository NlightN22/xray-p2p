package heartbeat

import "strings"

func payloadKey(payload Payload) string {
	if endpointID := strings.TrimSpace(payload.EndpointID); endpointID != "" {
		return "v1|" + endpointID
	}
	return entryKey(payload.Tag, payload.User)
}

func normalizeMode(mode Mode) Mode {
	switch mode {
	case ModeAuto, ModeDisabled, ModeRequired:
		return mode
	default:
		return ModeRequired
	}
}

func failureStatus(mode Mode, capability Capability, failures int, hasSucceeded bool) Status {
	if mode == ModeAuto && capability == CapabilityUnknown {
		if failures >= DiscoveryFailureThreshold {
			return StatusNotDetected
		}
		return StatusProbing
	}
	if failures >= HealthFailureThreshold {
		return StatusUnhealthy
	}
	if capability != CapabilityUnknown && hasSucceeded {
		return StatusHealthy
	}
	return StatusProbing
}

func normalizeCapability(capability Capability) Capability {
	switch capability {
	case CapabilityDetected:
		return CapabilityXP2PHeartbeat
	case CapabilityXP2PHeartbeat, CapabilityXP2PDiag:
		return capability
	default:
		return CapabilityUnknown
	}
}

func successfulCapability(current, observed Capability) Capability {
	current = normalizeCapability(current)
	observed = normalizeCapability(observed)
	if current == CapabilityXP2PHeartbeat {
		return current
	}
	if observed == CapabilityUnknown {
		return CapabilityXP2PHeartbeat
	}
	return observed
}

func normalizeFailure(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 256 {
		value = value[:256]
	}
	return value
}

func entryKey(tag, user string) string {
	key := strings.ToLower(strings.TrimSpace(tag))
	if key == "" {
		return ""
	}
	user = strings.ToLower(strings.TrimSpace(user))
	if user == "" {
		return key
	}
	return key + "|" + user
}
