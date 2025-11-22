package clientcmd

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type trojanLink struct {
	ServerAddress string
	ServerPort    string
	User          string
	Password      string
	ServerName    string
	ServerNameSet bool
	AllowInsecure bool
}

func parseTrojanLink(raw string) (trojanLink, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return trojanLink{}, fmt.Errorf("trojan link is empty")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return trojanLink{}, fmt.Errorf("parse trojan link: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "trojan") {
		return trojanLink{}, fmt.Errorf("unsupported scheme %q (expected trojan)", parsed.Scheme)
	}

	address := parsed.Hostname()
	if address == "" {
		return trojanLink{}, fmt.Errorf("missing host in trojan link")
	}

	portValue := parsed.Port()
	if portValue == "" {
		return trojanLink{}, fmt.Errorf("missing port in trojan link")
	}
	if _, err := strconv.Atoi(portValue); err != nil {
		return trojanLink{}, fmt.Errorf("invalid port %q in trojan link", portValue)
	}

	if parsed.User == nil {
		return trojanLink{}, fmt.Errorf("missing password in trojan link")
	}
	password := ""
	if pwd, ok := parsed.User.Password(); ok {
		password = strings.TrimSpace(pwd)
	} else {
		password = strings.TrimSpace(parsed.User.Username())
	}
	if password == "" {
		return trojanLink{}, fmt.Errorf("empty password in trojan link")
	}

	user, err := decodeTrojanUser(parsed)
	if err != nil {
		return trojanLink{}, err
	}

	query := parsed.Query()
	allowInsecure := false
	if rawAllow := strings.TrimSpace(query.Get("allowInsecure")); rawAllow != "" {
		val, convErr := parseBoolFlag(rawAllow)
		if convErr != nil {
			return trojanLink{}, fmt.Errorf("invalid allowInsecure value %q", rawAllow)
		}
		allowInsecure = val
	}

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

	return trojanLink{
		ServerAddress: address,
		ServerPort:    portValue,
		User:          user,
		Password:      password,
		ServerName:    serverName,
		ServerNameSet: serverNameSet,
		AllowInsecure: allowInsecure,
	}, nil
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
			return "", fmt.Errorf("decode trojan link user: %w", err)
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
		return "", fmt.Errorf("trojan link missing user/email (wrap the URL in quotes or escape '&' on Windows)")
	}
	return "", fmt.Errorf("trojan link missing user/email (expected #email or email query parameter)")
}
