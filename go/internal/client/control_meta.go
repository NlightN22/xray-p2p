package client

import (
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
)

func buildClientControlRuntime(cfg config.Config, endpoints []clientEndpointRecord) controlplane.Runtime {
	port := parseClientControlPort(cfg)
	users := make([]controlplane.AuthUser, 0, len(endpoints))
	for _, ep := range endpoints {
		user := strings.TrimSpace(ep.User)
		secret := strings.TrimSpace(ep.Password)
		if user == "" || secret == "" {
			continue
		}
		users = append(users, controlplane.AuthUser{Label: user, Credential: secret})
	}
	rt := controlplane.Runtime{
		Endpoint: controlplane.Endpoint{
			Scheme: "https",
			Port:   port,
		},
		AuthUsers: users,
	}
	if sub, ok := clientAppliedSubscription(endpoints); ok {
		rt.Subscription = sub
	}
	return rt
}

func parseClientControlPort(cfg config.Config) int {
	raw := strings.TrimSpace(cfg.Client.DiagPort)
	if raw == "" {
		raw = strings.TrimSpace(cfg.Server.Port)
	}
	if raw == "" {
		raw = DefaultDiagnosticsPort
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 || port > 65535 {
		return 62022
	}
	return port
}

func clientAppliedSubscription(endpoints []clientEndpointRecord) (controlplane.Subscription, bool) {
	for _, ep := range endpoints {
		if ep.Disabled {
			continue
		}
		host := strings.TrimSpace(ep.Hostname)
		if host == "" {
			host = strings.TrimSpace(ep.Address)
		}
		if host == "" || ep.Port <= 0 {
			continue
		}
		security := "tls"
		profile := strings.TrimSpace(ep.Profile)
		protocol := strings.TrimSpace(ep.Protocol)
		transport := strings.TrimSpace(ep.Transport)
		if profile == "" {
			profile = "trojan-tls"
		}
		if protocol == "" {
			protocol = "trojan"
		}
		if transport == "" {
			transport = "tcp"
		}
		tlsMeta := controlplane.TLSMetadata{
			ServerName:             strings.TrimSpace(ep.ServerName),
			PinnedPeerCertSHA256:   strings.TrimSpace(ep.PinnedPeerCertSHA256),
			VerifyPeerCertByName:   strings.TrimSpace(ep.VerifyPeerCertByName),
			ClientMayAllowInsecure: ep.AllowInsecure,
		}
		sub, err := controlplane.BuildSubscription(controlplane.Subscription{
			Profile:    profile,
			Protocol:   protocol,
			Transport:  transport,
			Security:   security,
			Host:       host,
			Port:       ep.Port,
			ServerName: strings.TrimSpace(ep.ServerName),
			TLS:        tlsMeta,
			Parameters: subscriptionParameters(ep),
		}, time.Now().UTC(), time.Hour)
		if err != nil {
			return controlplane.Subscription{}, false
		}
		return sub, true
	}
	return controlplane.Subscription{}, false
}

func subscriptionParameters(ep clientEndpointRecord) map[string]string {
	parameters := map[string]string{"tunnel_port": strconv.Itoa(ep.Port)}
	if strings.TrimSpace(ep.Flow) != "" {
		parameters["flow"] = strings.TrimSpace(ep.Flow)
	}
	return parameters
}
