package link

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type TrojanLink struct {
	ServerAddress    string
	ServerPort       string
	User             string
	Password         string
	ServerName       string
	ServerNameSet    bool
	ALPN             []string
	AllowInsecure    bool
	AllowInsecureSet bool
	PinnedPeerSHA256 string
	VerifyPeerName   string
}

func ParseTrojanLink(raw string) (TrojanLink, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return TrojanLink{}, fmt.Errorf("connection link is empty")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return TrojanLink{}, fmt.Errorf("parse connection link: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "trojan") {
		return TrojanLink{}, fmt.Errorf("unsupported scheme %q (expected connection link)", parsed.Scheme)
	}

	address := parsed.Hostname()
	if address == "" {
		return TrojanLink{}, fmt.Errorf("missing host in connection link")
	}

	portValue := parsed.Port()
	if portValue == "" {
		return TrojanLink{}, fmt.Errorf("missing port in connection link")
	}
	if _, err := strconv.Atoi(portValue); err != nil {
		return TrojanLink{}, fmt.Errorf("invalid port %q in connection link", portValue)
	}

	if parsed.User == nil {
		return TrojanLink{}, fmt.Errorf("missing password in connection link")
	}
	password := ""
	if pwd, ok := parsed.User.Password(); ok {
		password = strings.TrimSpace(pwd)
	} else {
		password = strings.TrimSpace(parsed.User.Username())
	}
	if password == "" {
		return TrojanLink{}, fmt.Errorf("empty password in connection link")
	}

	user, err := decodeTrojanUser(parsed)
	if err != nil {
		return TrojanLink{}, err
	}

	query := parsed.Query()
	allowInsecure := false
	allowInsecureSet := false
	if rawAllow := strings.TrimSpace(query.Get("allowInsecure")); rawAllow != "" {
		allowInsecureSet = true
		val, convErr := parseBoolFlag(rawAllow)
		if convErr != nil {
			return TrojanLink{}, fmt.Errorf("invalid allowInsecure value %q", rawAllow)
		}
		allowInsecure = val
	}

	pinnedPeerSHA256 := firstQuery(query, "xp2p_pin_sha256", "pinnedPeerCertSha256")
	verifyPeerName := firstQuery(query, "xp2p_verify_name", "verifyPeerCertByName")
	alpn := parseALPN(query)

	security := strings.ToLower(strings.TrimSpace(query.Get("security")))
	serverName := ""
	serverNameSet := false
	switch security {
	case "none":
		serverName = ""
		serverNameSet = true
		allowInsecure = false
	default:
		serverName = strings.TrimSpace(query.Get("sni"))
		if serverName == "" {
			serverName = address
		}
		serverNameSet = true
	}

	if security == "none" {
		pinnedPeerSHA256 = ""
		verifyPeerName = ""
		alpn = nil
	}
	if pinnedPeerSHA256 != "" && verifyPeerName == "" && serverName != "" {
		verifyPeerName = serverName
	}

	return TrojanLink{
		ServerAddress:    address,
		ServerPort:       portValue,
		User:             user,
		Password:         password,
		ServerName:       serverName,
		ServerNameSet:    serverNameSet,
		ALPN:             alpn,
		AllowInsecure:    allowInsecure,
		AllowInsecureSet: allowInsecureSet,
		PinnedPeerSHA256: pinnedPeerSHA256,
		VerifyPeerName:   verifyPeerName,
	}, nil
}

func firstQuery(values url.Values, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func parseALPN(values url.Values) []string {
	rawValues, ok := values["alpn"]
	if !ok || len(rawValues) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(rawValues))
	result := make([]string, 0, len(rawValues))
	for _, raw := range rawValues {
		for _, part := range strings.Split(raw, ",") {
			clean := strings.TrimSpace(part)
			if clean == "" {
				continue
			}
			key := strings.ToLower(clean)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, clean)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func parseBoolFlag(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
}

func decodeTrojanUser(u *url.URL) (string, error) {
	fragment := strings.TrimSpace(u.Fragment)
	if fragment != "" {
		decoded, err := url.PathUnescape(fragment)
		if err != nil {
			return "", fmt.Errorf("decode connection link user: %w", err)
		}
		decoded = strings.TrimSpace(decoded)
		if decoded != "" {
			return decoded, nil
		}
	}

	candidates := []string{
		"email",
		"user",
		"username",
		"name",
		"remark",
		"remarks",
		"peer",
	}
	query := u.Query()
	for _, key := range candidates {
		if val := strings.TrimSpace(query.Get(key)); val != "" {
			return val, nil
		}
	}

	if strings.Contains(u.RawQuery, "&") && !strings.Contains(u.RawPath, "#") && !strings.Contains(u.Fragment, "#") {
		return "", fmt.Errorf("connection link missing user/email (wrap the URL in quotes or escape '&' on Windows)")
	}
	return "", fmt.Errorf("connection link missing user/email (expected #email or email query parameter)")
}
