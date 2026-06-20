package tunnel

import "fmt"

// XrayOutbound renders the protocol-specific outbound fragment for an endpoint.
func XrayOutbound(link Link, tag string) (map[string]any, error) {
	endpoint, err := Normalize(link.Endpoint)
	if err != nil {
		return nil, err
	}
	if link.User.Credential == "" {
		return nil, fmt.Errorf("connection credential is required")
	}
	user := map[string]any{"email": link.User.UserLabel}
	if endpoint.Protocol == "trojan" {
		user["password"] = link.User.Credential
	} else {
		user["id"] = link.User.Credential
		if flow := endpoint.Metadata["flow"]; flow != "" {
			user["flow"] = flow
		}
	}
	settings := map[string]any{"vnext": []any{map[string]any{"address": endpoint.Host, "port": endpoint.Port, "users": []any{user}}}}
	if endpoint.Protocol == "trojan" {
		settings = map[string]any{"servers": []any{map[string]any{"address": endpoint.Host, "port": endpoint.Port, "password": link.User.Credential, "email": link.User.UserLabel}}}
	}
	return map[string]any{"tag": tag, "protocol": endpoint.Protocol, "settings": settings, "streamSettings": streamSettings(endpoint)}, nil
}

// XrayInboundUser renders the protocol-specific user fragment for an inbound.
func XrayInboundUser(protocol string, user User) (map[string]any, error) {
	if user.Credential == "" {
		return nil, fmt.Errorf("connection credential is required")
	}
	switch protocol {
	case "trojan":
		return map[string]any{"email": user.UserLabel, "password": user.Credential}, nil
	case "vless":
		return map[string]any{"email": user.UserLabel, "id": user.Credential}, nil
	default:
		return nil, fmt.Errorf("unsupported tunnel protocol %q", protocol)
	}
}

func streamSettings(endpoint Endpoint) map[string]any {
	stream := map[string]any{"network": endpoint.Transport, "security": endpoint.Security}
	if endpoint.Security == "tls" {
		stream["tlsSettings"] = map[string]any{"serverName": endpoint.ServerName, "allowInsecure": endpoint.TLS.AllowInsecure, "alpn": endpoint.TLS.ALPN}
	}
	return stream
}
