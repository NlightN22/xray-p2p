package forward

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
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
	ListenAddress string   `json:"listen_address"`
	ListenPort    int      `json:"listen_port"`
	TargetIP      string   `json:"target_ip"`
	TargetPort    int      `json:"target_port"`
	Protocol      Protocol `json:"protocol"`
	Tag           string   `json:"tag"`
	Remark        string   `json:"remark"`
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
			"address":        r.TargetIP,
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

// Target renders the target IP:port combination.
func (r Rule) Target() string {
	return net.JoinHostPort(r.TargetIP, strconv.Itoa(r.TargetPort))
}

// BuildRemark renders the canonical remark for a forward entry.
func BuildRemark(ip string, port int) string {
	return fmt.Sprintf("forward:%s:%d", ip, port)
}

// TagForPort renders the canonical inbound tag.
func TagForPort(port int) string {
	return fmt.Sprintf("in_%d", port)
}

// ParseTarget validates IP:port syntax and returns normalized components.
func ParseTarget(value string) (netip.Addr, int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return netip.Addr{}, 0, errors.New("xp2p: --target is required")
	}
	addrPort, err := netip.ParseAddrPort(trimmed)
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("xp2p: invalid --target %q: %w", value, err)
	}
	return addrPort.Addr(), int(addrPort.Port()), nil
}

// NormalizeListenAddress validates the listen address or falls back to default.
func NormalizeListenAddress(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = DefaultListenAddress
	}
	addr, err := netip.ParseAddr(trimmed)
	if err != nil {
		return "", fmt.Errorf("xp2p: invalid --listen address %q: %w", value, err)
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
		return "", fmt.Errorf("xp2p: invalid --proto value %q (expected tcp, udp, or both)", value)
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

// ErrPortUnavailable indicates the address is already bound.
var ErrPortUnavailable = errors.New("xp2p: port unavailable")

// CheckPort ensures that the provided listener address is available for all requested protocols.
func CheckPort(listen string, port int, proto Protocol) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("xp2p: invalid --listen-port %d", port)
	}
	if proto.RequiresTCP() {
		if err := probeTCP(listen, port); err != nil {
			return err
		}
	}
	if proto.RequiresUDP() {
		if err := probeUDP(listen, port); err != nil {
			return err
		}
	}
	return nil
}

// FindAvailablePort searches for the first available port from start..65535 skipping reserved entries.
func FindAvailablePort(listen string, start int, proto Protocol, reserved map[int]struct{}) (int, error) {
	if start < 1 {
		start = DefaultBasePort
	}
	for port := start; port <= 65535; port++ {
		if _, taken := reserved[port]; taken {
			continue
		}
		if err := CheckPort(listen, port, proto); err != nil {
			if errors.Is(err, ErrPortUnavailable) {
				continue
			}
			return 0, err
		}
		return port, nil
	}
	return 0, fmt.Errorf("xp2p: no free ports available from %d to 65535", start)
}

func probeTCP(listen string, port int) error {
	addr := net.JoinHostPort(listen, strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if isAddrInUse(err) {
			return ErrPortUnavailable
		}
		return fmt.Errorf("xp2p: bind TCP %s: %w", addr, err)
	}
	return ln.Close()
}

func probeUDP(listen string, port int) error {
	ip := net.ParseIP(listen)
	if ip == nil {
		return fmt.Errorf("xp2p: invalid listen address %q", listen)
	}
	addr := &net.UDPAddr{IP: ip, Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		if isAddrInUse(err) {
			return ErrPortUnavailable
		}
		return fmt.Errorf("xp2p: bind UDP %s: %w", addr.String(), err)
	}
	return conn.Close()
}

func isAddrInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	var syscallErr *os.SyscallError
	if !errors.As(opErr.Err, &syscallErr) {
		return false
	}
	return errors.Is(syscallErr.Err, syscall.EADDRINUSE)
}

// MatchesRedirect reports whether any redirect rule routes the supplied IP.
func MatchesRedirect(rules []redirect.Rule, addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	for _, rule := range rules {
		if rule.Kind() != redirect.KindCIDR {
			continue
		}
		prefix, err := netip.ParsePrefix(rule.CIDR)
		if err != nil {
			continue
		}
		if !prefix.Contains(addr) {
			continue
		}
		return true
	}
	return false
}
