package forward

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// DefaultListenAddress is used when --listen is omitted.
const DefaultListenAddress = "127.0.0.1"

// DefaultBasePort is the first port probed when --listen-port is omitted.
const DefaultBasePort = 53331

// Protocol controls which transports the dokodemo-door listener should accept.
type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
	ProtocolBoth Protocol = "both"
)

// Rule stores all metadata required to manage dokodemo-door forwards.
type Rule struct {
	ListenAddress string   `json:"listen_address" toml:"listen_address"`
	ListenPort    int      `json:"listen_port" toml:"listen_port"`
	TargetHost    string   `json:"target_host" toml:"target_host"`
	TargetPort    int      `json:"target_port" toml:"target_port"`
	Protocol      Protocol `json:"protocol" toml:"protocol"`
	Tag           string   `json:"tag" toml:"tag"`
	Remark        string   `json:"remark" toml:"remark"`
}

// Selector allows lookups using listen port, tag, or remark.
type Selector struct {
	ListenPort int
	Tag        string
	Remark     string
}

// Matches reports whether the selector matches the provided rule.
func (s Selector) Matches(rule Rule) bool {
	match := false
	if s.ListenPort > 0 {
		if rule.ListenPort != s.ListenPort {
			return false
		}
		match = true
	}
	if trimmed := strings.TrimSpace(s.Tag); trimmed != "" {
		if !strings.EqualFold(rule.Tag, trimmed) {
			return false
		}
		match = true
	}
	if trimmed := strings.TrimSpace(s.Remark); trimmed != "" {
		if !strings.EqualFold(rule.Remark, trimmed) {
			return false
		}
		match = true
	}
	return match
}

// Empty reports whether the selector has any criteria.
func (s Selector) Empty() bool {
	return s.ListenPort <= 0 && strings.TrimSpace(s.Tag) == "" && strings.TrimSpace(s.Remark) == ""
}

// InboundMap renders the dokodemo-door JSON object for the rule.
func (r Rule) InboundMap() map[string]any {
	return map[string]any{
		"remark":   r.Remark,
		"tag":      r.Tag,
		"listen":   r.ListenAddress,
		"port":     r.ListenPort,
		"protocol": "dokodemo-door",
		"settings": map[string]any{
			"address":        r.TargetHost,
			"port":           r.TargetPort,
			"network":        r.NetworkValue(),
			"followRedirect": false,
		},
	}
}

// NetworkValue produces the XRAY network string for the rule protocols.
func (r Rule) NetworkValue() string {
	switch strings.ToLower(string(r.Protocol)) {
	case string(ProtocolTCP):
		return "tcp"
	case string(ProtocolUDP):
		return "udp"
	default:
		return "tcp,udp"
	}
}

// Target renders the target host:port combination.
func (r Rule) Target() string {
	return net.JoinHostPort(r.TargetHost, strconv.Itoa(r.TargetPort))
}

// BuildRemark renders the canonical remark for a forward entry.
func BuildRemark(host string, port int) string {
	return fmt.Sprintf("forward:%s:%d", host, port)
}

// TagForPort renders the canonical inbound tag.
func TagForPort(port int) string {
	return fmt.Sprintf("in_%d", port)
}

// ParseTarget validates host:port syntax and returns normalized components.
func ParseTarget(value string) (string, int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", 0, errors.New("--target is required")
	}
	host, portValue, err := net.SplitHostPort(trimmed)
	if err != nil {
		return "", 0, fmt.Errorf("invalid --target %q: %w", value, err)
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", 0, errors.New("--target host is required")
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("invalid --target port %q", portValue)
	}
	return host, port, nil
}

// NormalizeListenAddress validates the listen address or falls back to default.
func NormalizeListenAddress(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = DefaultListenAddress
	}
	addr, err := netip.ParseAddr(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid --listen address %q: %w", value, err)
	}
	return addr.String(), nil
}

// ParseProtocol converts user input into a Protocol value.
func ParseProtocol(value string) (Protocol, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ProtocolBoth, nil
	}
	switch trimmed {
	case string(ProtocolTCP):
		return ProtocolTCP, nil
	case string(ProtocolUDP):
		return ProtocolUDP, nil
	case string(ProtocolBoth):
		return ProtocolBoth, nil
	default:
		return "", fmt.Errorf("invalid --proto value %q (expected tcp, udp, or both)", value)
	}
}

// RequiresTCP reports whether the listener needs a TCP socket.
func (p Protocol) RequiresTCP() bool {
	switch strings.ToLower(string(p)) {
	case string(ProtocolUDP):
		return false
	default:
		return true
	}
}

// RequiresUDP reports whether the listener needs a UDP socket.
func (p Protocol) RequiresUDP() bool {
	switch strings.ToLower(string(p)) {
	case string(ProtocolTCP):
		return false
	default:
		return true
	}
}
