package tunnel

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Link struct {
	Endpoint Endpoint
	User     User
	Unknown  url.Values
}

func ParseLink(raw string) (Link, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Link{}, fmt.Errorf("parse connection link: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "trojan":
		return parseTrojan(u)
	case "vless":
		return parseVLESS(u)
	default:
		return Link{}, fmt.Errorf("unsupported connection link scheme %q", u.Scheme)
	}
}

func RenderLink(link Link) (string, error) {
	endpoint, err := Normalize(link.Endpoint)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(ActiveCredential(link.User)) == "" {
		return "", fmt.Errorf("connection credential is required")
	}
	if strings.TrimSpace(endpoint.Host) == "" || endpoint.Port < 1 || endpoint.Port > 65535 {
		return "", fmt.Errorf("connection host and port are required")
	}
	if strings.EqualFold(endpoint.Protocol, "trojan") {
		return renderTrojan(endpoint, link.User, link.Unknown), nil
	}
	return renderVLESS(endpoint, link.User, link.Unknown), nil
}

func parseTrojan(u *url.URL) (Link, error) {
	credential, host, port, err := parseAuthority(u)
	if err != nil {
		return Link{}, err
	}
	query := u.Query()
	endpoint, _ := DefaultProfile(ProfileTrojanTLS)
	endpoint.Host, endpoint.Port = host, port
	endpoint.Transport = first(query.Get("type"), endpoint.Transport)
	endpoint.Security = first(query.Get("security"), endpoint.Security)
	endpoint.ServerName = first(query.Get("sni"), host)
	endpoint.TLS = tlsFromQuery(query)
	endpoint, err = Normalize(endpoint)
	if err != nil {
		return Link{}, err
	}
	return Link{Endpoint: endpoint, User: User{UserLabel: labelFromURL(u), Credential: credential}, Unknown: unknown(query, trojanKnown)}, nil
}

func parseVLESS(u *url.URL) (Link, error) {
	credential, host, port, err := parseAuthority(u)
	if err != nil {
		return Link{}, err
	}
	if err := ValidateVLESSCredential(credential); err != nil {
		return Link{}, err
	}
	query := u.Query()
	endpoint, _ := DefaultProfile(ProfileVLESSTLSVision)
	endpoint.Host, endpoint.Port = host, port
	endpoint.Transport = first(query.Get("type"), endpoint.Transport)
	endpoint.Security = first(query.Get("security"), endpoint.Security)
	endpoint.ServerName = first(query.Get("sni"), host)
	endpoint.TLS = tlsFromQuery(query)
	if flow := strings.TrimSpace(query.Get("flow")); flow != "" {
		endpoint.Metadata["flow"] = flow
	}
	endpoint, err = Normalize(endpoint)
	if err != nil {
		return Link{}, err
	}
	return Link{Endpoint: endpoint, User: User{UserLabel: labelFromURL(u), Credential: credential}, Unknown: unknown(query, vlessKnown)}, nil
}

func parseAuthority(u *url.URL) (string, string, int, error) {
	if u.User == nil || strings.TrimSpace(u.User.Username()) == "" {
		return "", "", 0, fmt.Errorf("connection credential is required")
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return "", "", 0, fmt.Errorf("connection host is required")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", "", 0, fmt.Errorf("invalid connection port %q", u.Port())
	}
	return strings.TrimSpace(u.User.Username()), host, port, nil
}

func renderTrojan(endpoint Endpoint, user User, values url.Values) string {
	u := newURL("trojan", endpoint, user, values)
	return u.String()
}

func renderVLESS(endpoint Endpoint, user User, values url.Values) string {
	values = clone(values)
	if flow := strings.TrimSpace(endpoint.Metadata["flow"]); flow != "" {
		values.Set("flow", flow)
	}
	values.Set("encryption", "none")
	u := newURL("vless", endpoint, user, values)
	return u.String()
}

func newURL(scheme string, endpoint Endpoint, user User, values url.Values) *url.URL {
	values = clone(values)
	values.Set("security", endpoint.Security)
	values.Set("type", endpoint.Transport)
	if endpoint.ServerName != "" {
		values.Set("sni", endpoint.ServerName)
	}
	writeTLS(values, endpoint.TLS)
	u := &url.URL{Scheme: scheme, Host: net.JoinHostPort(endpoint.Host, strconv.Itoa(endpoint.Port)), User: url.User(ActiveCredential(user)), RawQuery: values.Encode()}
	if strings.TrimSpace(user.UserLabel) != "" {
		u.Fragment = user.UserLabel
	}
	return u
}

func tlsFromQuery(values url.Values) TLSMetadata {
	return TLSMetadata{ALPN: splitALPN(values["alpn"]), AllowInsecure: boolValue(values.Get("allowInsecure")), PinnedPeerCertSHA256: strings.TrimSpace(values.Get("pinnedPeerCertSha256")), VerifyPeerCertByName: strings.TrimSpace(values.Get("verifyPeerCertByName"))}
}
func writeTLS(values url.Values, tls TLSMetadata) {
	if len(tls.ALPN) > 0 {
		values.Set("alpn", strings.Join(tls.ALPN, ","))
	}
	if tls.AllowInsecure {
		values.Set("allowInsecure", "1")
	}
	if tls.PinnedPeerCertSHA256 != "" {
		values.Set("pinnedPeerCertSha256", tls.PinnedPeerCertSHA256)
	}
	if tls.VerifyPeerCertByName != "" {
		values.Set("verifyPeerCertByName", tls.VerifyPeerCertByName)
	}
}
func labelFromURL(u *url.URL) string { return strings.TrimSpace(u.Fragment) }
func first(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
func boolValue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "1" || value == "true"
}
func splitALPN(values []string) []string {
	var out []string
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
	}
	return out
}
func clone(values url.Values) url.Values {
	out := make(url.Values, len(values))
	for key, value := range values {
		out[key] = append([]string(nil), value...)
	}
	return out
}
func unknown(values url.Values, known map[string]struct{}) url.Values {
	out := url.Values{}
	for key, value := range values {
		if _, ok := known[key]; !ok {
			out[key] = append([]string(nil), value...)
		}
	}
	return out
}

var trojanKnown = known("security", "type", "sni", "alpn", "allowInsecure", "pinnedPeerCertSha256", "verifyPeerCertByName")
var vlessKnown = known("security", "type", "sni", "alpn", "allowInsecure", "pinnedPeerCertSha256", "verifyPeerCertByName", "flow", "encryption")

func known(keys ...string) map[string]struct{} {
	values := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		values[key] = struct{}{}
	}
	return values
}
