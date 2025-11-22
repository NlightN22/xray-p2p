package forward

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"syscall"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestSelectorMatches(t *testing.T) {
	rule := Rule{ListenPort: 60022, Tag: "in_60022", Remark: "ssh"}

	sel := Selector{ListenPort: 60022}
	if !sel.Matches(rule) {
		t.Fatalf("selector by port should match")
	}
	sel = Selector{ListenPort: 22}
	if sel.Matches(rule) {
		t.Fatalf("unexpected match by wrong port")
	}
	sel = Selector{Tag: "IN_60022"}
	if !sel.Matches(rule) {
		t.Fatalf("selector by tag should match case-insensitively")
	}
	sel = Selector{Remark: "other"}
	if sel.Matches(rule) {
		t.Fatalf("remark mismatch should be false")
	}
	sel = Selector{ListenPort: 60022, Tag: "in_60022"}
	if !sel.Matches(rule) {
		t.Fatalf("combined selector should match")
	}
	sel = Selector{Remark: "ssh"}
	if !sel.Matches(rule) {
		t.Fatalf("selector by remark should match")
	}
	sel = Selector{}
	if sel.Matches(rule) {
		t.Fatalf("empty selector must not match")
	}
}

func TestRuleHelpers(t *testing.T) {
	rule := Rule{
		ListenAddress: "127.0.0.1",
		ListenPort:    60022,
		TargetIP:      "192.0.2.10",
		TargetPort:    22,
		Protocol:      ProtocolBoth,
		Tag:           "in_60022",
		Remark:        "ssh",
	}
	if v := rule.Target(); v != "192.0.2.10:22" {
		t.Fatalf("Target() = %s", v)
	}
	if BuildRemark(rule.TargetIP, rule.TargetPort) != "forward:192.0.2.10:22" {
		t.Fatalf("BuildRemark mismatch")
	}
	if TagForPort(12345) != "in_12345" {
		t.Fatalf("TagForPort mismatch")
	}
	inbound := rule.InboundMap()
	if inbound["protocol"] != "dokodemo-door" {
		t.Fatalf("InboundMap missing protocol: %v", inbound)
	}
	if rule.NetworkValue() != "tcp,udp" {
		t.Fatalf("NetworkValue for both expected tcp,udp")
	}
	rule.Protocol = ProtocolUDP
	if rule.NetworkValue() != "udp" || rule.Protocol.RequiresTCP() {
		t.Fatalf("UDP protocol should not require TCP")
	}
	if !rule.Protocol.RequiresUDP() {
		t.Fatalf("UDP protocol should require UDP")
	}
}

func TestParseTarget(t *testing.T) {
	addr, port, err := ParseTarget("192.0.2.10:443")
	if err != nil {
		t.Fatalf("ParseTarget error: %v", err)
	}
	if addr.String() != "192.0.2.10" || port != 443 {
		t.Fatalf("ParseTarget mismatch %s %d", addr, port)
	}
	if _, _, err := ParseTarget("not-a-port"); err == nil {
		t.Fatalf("expected error for invalid target")
	}
}

func TestNormalizeListenAddress(t *testing.T) {
	addr, err := NormalizeListenAddress("")
	if err != nil || addr != DefaultListenAddress {
		t.Fatalf("NormalizeListenAddress default failed: %s %v", addr, err)
	}
	if _, err := NormalizeListenAddress("bad addr"); err == nil {
		t.Fatalf("expected error for invalid listen address")
	}
}

func TestParseProtocol(t *testing.T) {
	cases := map[string]Protocol{
		"":     ProtocolBoth,
		"TCP":  ProtocolTCP,
		"udp":  ProtocolUDP,
		"both": ProtocolBoth,
	}
	for input, want := range cases {
		got, err := ParseProtocol(input)
		if err != nil || got != want {
			t.Fatalf("ParseProtocol(%q) = %v, %v", input, got, err)
		}
	}
	if _, err := ParseProtocol("bad"); err == nil {
		t.Fatalf("expected error for invalid protocol")
	}
}

func TestCheckPortAndFindAvailablePort(t *testing.T) {
	listen := "127.0.0.1"
	tcpLn, err := net.Listen("tcp", net.JoinHostPort(listen, "0"))
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	port := tcpLn.Addr().(*net.TCPAddr).Port
	if err := CheckPort(listen, port, ProtocolTCP); err == nil {
		t.Fatalf("CheckPort expected error when port is busy")
	}
	_ = tcpLn.Close()
	if err := CheckPort(listen, port, ProtocolBoth); err != nil {
		t.Fatalf("CheckPort after release failed: %v", err)
	}

	startPort := port + 5
	reserved := map[int]struct{}{startPort: {}, startPort + 1: {}}
	found, err := FindAvailablePort(listen, startPort, ProtocolTCP, reserved)
	if err != nil {
		t.Fatalf("FindAvailablePort error: %v", err)
	}
	if found <= startPort+1 {
		t.Fatalf("FindAvailablePort returned unexpected port %d", found)
	}
}

func TestIsAddrInUse(t *testing.T) {
	if !isAddrInUse(syscall.EADDRINUSE) {
		t.Fatalf("direct syscall should be addr in use")
	}
	opErr := &net.OpError{
		Err: &os.SyscallError{Err: syscall.EADDRINUSE},
	}
	if !isAddrInUse(opErr) {
		t.Fatalf("wrapped error should be addr in use")
	}
	if isAddrInUse(errors.New("other error")) {
		t.Fatalf("unexpected true for other error")
	}
}

func TestMatchesRedirect(t *testing.T) {
	rules := []redirect.Rule{
		{CIDR: "10.0.0.0/24"},
		{Domain: "example.com"},
	}
	ip, _ := netip.ParseAddr("10.0.0.5")
	if !MatchesRedirect(rules, ip) {
		t.Fatalf("expected match for CIDR rule")
	}
	if MatchesRedirect(rules, netip.Addr{}) {
		t.Fatalf("invalid addr should not match")
	}
	other, _ := netip.ParseAddr("192.0.2.1")
	if MatchesRedirect(rules, other) {
		t.Fatalf("unexpected match for other address")
	}
}
