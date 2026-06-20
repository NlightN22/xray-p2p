package tunnel

import "fmt"

func ActiveCredential(user User) string {
	if user.ActiveCredential != "" {
		return user.ActiveCredential
	}
	return user.Credential
}

// XrayOutbound renders the protocol-specific outbound fragment for an endpoint.
func XrayOutbound(link Link, tag string) (map[string]any, error) {
	endpoint, err := Normalize(link.Endpoint)
	if err != nil {
		return nil, err
	}
	credential := ActiveCredential(link.User)
	if credential == "" {
		return nil, fmt.Errorf("connection credential is required")
	}
	if endpoint.Protocol == "vless" {
		if err := ValidateVLESSCredential(credential); err != nil {
			return nil, err
		}
	}
	user := map[string]any{"email": link.User.UserLabel}
	if endpoint.Protocol == "trojan" {
		user["password"] = credential
	} else {
		user["id"] = credential
		if flow := endpoint.Metadata["flow"]; flow != "" {
			user["flow"] = flow
		}
	}
	settings := map[string]any{"vnext": []any{map[string]any{"address": endpoint.Host, "port": endpoint.Port, "users": []any{user}}}}
	if endpoint.Protocol == "trojan" {
		settings = map[string]any{"servers": []any{map[string]any{"address": endpoint.Host, "port": endpoint.Port, "password": credential, "email": link.User.UserLabel}}}
	}
	return map[string]any{"tag": tag, "protocol": endpoint.Protocol, "settings": settings, "streamSettings": streamSettings(endpoint)}, nil
}

// XrayInboundUser renders the protocol-specific user fragment for an inbound.
func XrayInboundUser(protocol string, user User) (map[string]any, error) {
	credential := ActiveCredential(user)
	if credential == "" {
		return nil, fmt.Errorf("connection credential is required")
	}
	switch protocol {
	case "trojan":
		return map[string]any{"email": user.UserLabel, "password": credential}, nil
	case "vless":
		if err := ValidateVLESSCredential(credential); err != nil {
			return nil, err
		}
		return map[string]any{"email": user.UserLabel, "id": credential}, nil
	default:
		return nil, fmt.Errorf("unsupported tunnel protocol %q", protocol)
	}
}

func streamSettings(endpoint Endpoint) map[string]any {
	stream := map[string]any{"network": endpoint.Transport, "security": endpoint.Security}
	if endpoint.Security == "tls" {
		tlsSettings := map[string]any{"serverName": endpoint.ServerName, "alpn": endpoint.TLS.ALPN}
		if endpoint.TLS.PinnedPeerCertSHA256 != "" {
			tlsSettings["pinnedPeerCertSha256"] = endpoint.TLS.PinnedPeerCertSHA256
		}
		if endpoint.TLS.VerifyPeerCertByName != "" {
			tlsSettings["verifyPeerCertByName"] = endpoint.TLS.VerifyPeerCertByName
		}
		stream["tlsSettings"] = tlsSettings
	}
	return stream
}
