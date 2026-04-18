package root

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func stringifyListen(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("invalid SOCKS listen value of type %T", value)
	}
	return strings.TrimSpace(str), nil
}

func parseInboundPort(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", errors.New("SOCKS inbound is missing \"port\"")
	case json.Number:
		return normalizePortString(v.String())
	case float64:
		return normalizePortInt(int(v), v == float64(int(v)))
	case string:
		return normalizePortString(v)
	default:
		return "", fmt.Errorf("invalid SOCKS inbound port value of type %T", value)
	}
}

func normalizePortString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("SOCKS inbound port is empty")
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return "", fmt.Errorf("invalid SOCKS inbound port %q: %w", raw, err)
	}
	if val <= 0 || val > 65535 {
		return "", fmt.Errorf("invalid SOCKS inbound port %q: must be within 1-65535", raw)
	}
	return strconv.Itoa(val), nil
}

func normalizePortInt(value int, exact bool) (string, error) {
	if !exact {
		return "", fmt.Errorf("invalid SOCKS inbound port %d: not an integer", value)
	}
	if value <= 0 || value > 65535 {
		return "", fmt.Errorf("invalid SOCKS inbound port %d: must be within 1-65535", value)
	}
	return strconv.Itoa(value), nil
}

func isIgnorableServerResolveError(err error) bool {
	return errors.Is(err, server.ErrServerReverseMissing) ||
		errors.Is(err, server.ErrServerReverseNotFound) ||
		errors.Is(err, server.ErrServerReverseNotSpecified) ||
		errors.Is(err, server.ErrServerReverseAmbiguous)
}

func splitHostPort(value string) (string, string, error) {
	if strings.HasPrefix(value, "[") {
		host, port, err := net.SplitHostPort(value)
		if err != nil {
			return "", "", fmt.Errorf("invalid SOCKS proxy address %q: %w", value, err)
		}
		if err := validatePort(port); err != nil {
			return "", "", err
		}
		return host, port, nil
	}

	idx := strings.LastIndex(value, ":")
	if idx == -1 {
		return "", "", fmt.Errorf("invalid SOCKS proxy address %q: expected host:port", value)
	}
	host := strings.TrimSpace(value[:idx])
	port := strings.TrimSpace(value[idx+1:])
	if host == "" {
		return "", "", fmt.Errorf("invalid SOCKS proxy address %q: host is empty", value)
	}
	if err := validatePort(port); err != nil {
		return "", "", err
	}
	return host, port, nil
}

func validatePort(port string) error {
	p, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid SOCKS proxy port %q: %w", port, err)
	}
	if p <= 0 || p > 65535 {
		return fmt.Errorf("invalid SOCKS proxy port %q: must be within 1-65535", port)
	}
	return nil
}
