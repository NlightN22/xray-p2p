package xrayapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	commonnet "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonnet"
	routerconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/routerconfig"
)

var supportedRoutingRuleFields = map[string]struct{}{
	"type":        {},
	"ruleTag":     {},
	"outboundTag": {},
	"ip":          {},
	"domain":      {},
	"domains":     {},
	"port":        {},
	"inboundTag":  {},
	"user":        {},
	"protocol":    {},
}

func routingRuleFromMap(rule map[string]any) (*routerconfig.RoutingRule, error) {
	for key := range rule {
		if _, ok := supportedRoutingRuleFields[key]; !ok {
			return nil, fmt.Errorf("unsupported routing rule field %q", key)
		}
	}
	if typ, _ := rule["type"].(string); !strings.EqualFold(strings.TrimSpace(typ), "field") {
		return nil, errors.New("only field routing rules are supported")
	}
	ruleTag, _ := rule["ruleTag"].(string)
	ruleTag = strings.TrimSpace(ruleTag)
	if ruleTag == "" {
		return nil, errors.New("routing ruleTag is required")
	}
	outboundTag, _ := rule["outboundTag"].(string)
	outboundTag = strings.TrimSpace(outboundTag)
	if outboundTag == "" {
		return nil, errors.New("routing outboundTag is required")
	}

	result := &routerconfig.RoutingRule{
		RuleTag:   ruleTag,
		TargetTag: &routerconfig.RoutingRule_Tag{Tag: outboundTag},
	}
	domains, err := parseRoutingDomains(firstNonNil(rule["domain"], rule["domains"]))
	if err != nil {
		return nil, err
	}
	result.Domain = domains
	geoips, err := parseRoutingIPs(rule["ip"])
	if err != nil {
		return nil, err
	}
	result.Geoip = geoips
	portList, err := parsePortList(rule["port"])
	if err != nil {
		return nil, err
	}
	result.PortList = portList
	result.InboundTag = stringsFromAny(rule["inboundTag"])
	result.UserEmail = stringsFromAny(rule["user"])
	result.Protocol = stringsFromAny(rule["protocol"])
	return result, nil
}

func parseRoutingDomains(raw any) ([]*routerconfig.Domain, error) {
	values := stringsFromAny(raw)
	result := make([]*routerconfig.Domain, 0, len(values))
	for _, value := range values {
		typ := routerconfig.Domain_Plain
		domain := value
		if prefix, rest, ok := strings.Cut(value, ":"); ok {
			switch strings.ToLower(prefix) {
			case "full":
				typ = routerconfig.Domain_Full
				domain = rest
			case "domain":
				typ = routerconfig.Domain_Domain
				domain = rest
			case "regexp", "regex":
				typ = routerconfig.Domain_Regex
				domain = rest
			default:
				domain = value
			}
		}
		domain = strings.TrimSpace(domain)
		if domain == "" {
			return nil, errors.New("routing domain value is empty")
		}
		result = append(result, &routerconfig.Domain{Type: typ, Value: domain})
	}
	return result, nil
}

func parseRoutingIPs(raw any) ([]*routerconfig.GeoIP, error) {
	values := stringsFromAny(raw)
	if len(values) == 0 {
		return nil, nil
	}
	cidrs := make([]*routerconfig.CIDR, 0, len(values))
	for _, value := range values {
		ip, network, err := net.ParseCIDR(value)
		if err != nil {
			parsedIP := net.ParseIP(value)
			if parsedIP == nil {
				return nil, fmt.Errorf("invalid routing ip %q", value)
			}
			ip = parsedIP
			prefix := 32
			if parsedIP.To4() == nil {
				prefix = 128
			}
			cidrs = append(cidrs, &routerconfig.CIDR{Ip: ipBytes(ip), Prefix: uint32(prefix)})
			continue
		}
		ones, _ := network.Mask.Size()
		cidrs = append(cidrs, &routerconfig.CIDR{Ip: ipBytes(ip), Prefix: uint32(ones)})
	}
	return []*routerconfig.GeoIP{{Cidr: cidrs}}, nil
}

func parsePortList(raw any) (*commonnet.PortList, error) {
	values := stringsFromAny(raw)
	if len(values) == 0 {
		return nil, nil
	}
	ranges := make([]*commonnet.PortRange, 0, len(values))
	for _, value := range values {
		from, to, err := parsePortRange(value)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, &commonnet.PortRange{From: from, To: to})
	}
	return &commonnet.PortList{Range: ranges}, nil
}

func parsePortRange(value string) (uint32, uint32, error) {
	left, right, ok := strings.Cut(strings.TrimSpace(value), "-")
	if !ok {
		port, err := parsePort(left)
		return port, port, err
	}
	from, err := parsePort(left)
	if err != nil {
		return 0, 0, err
	}
	to, err := parsePort(right)
	if err != nil {
		return 0, 0, err
	}
	if from > to {
		return 0, 0, fmt.Errorf("invalid port range %q", value)
	}
	return from, to, nil
}

func parsePort(value string) (uint32, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid port %q", value)
	}
	return uint32(n), nil
}

func stringsFromAny(raw any) []string {
	switch value := raw.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []string{strings.TrimSpace(value)}
	case []string:
		result := make([]string, 0, len(value))
		for _, item := range value {
			if strings.TrimSpace(item) != "" {
				result = append(result, strings.TrimSpace(item))
			}
		}
		return result
	case []any:
		result := make([]string, 0, len(value))
		for _, item := range value {
			if str, ok := item.(string); ok && strings.TrimSpace(str) != "" {
				result = append(result, strings.TrimSpace(str))
			}
		}
		return result
	default:
		return nil
	}
}

func ipBytes(ip net.IP) []byte {
	if v4 := ip.To4(); v4 != nil {
		return []byte(v4)
	}
	return []byte(ip.To16())
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func cloneRuleMap(rule map[string]any) map[string]any {
	buf, err := json.Marshal(rule)
	if err != nil {
		return rule
	}
	var cloned map[string]any
	if err := json.Unmarshal(buf, &cloned); err != nil {
		return rule
	}
	return cloned
}
