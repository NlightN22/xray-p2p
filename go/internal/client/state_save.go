package client

import (
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func (s *clientInstallState) normalize() {
	for index := range s.Endpoints {
		endpoint := &s.Endpoints[index]
		if endpoint.Profile == "" {
			endpoint.Profile = "trojan-tls"
		}
		if endpoint.Protocol == "" {
			endpoint.Protocol = "trojan"
		}
		if endpoint.Transport == "" {
			endpoint.Transport = "tcp"
		}
		if endpoint.Security == "" {
			endpoint.Security = "tls"
		}
		if endpoint.HeartbeatMode == "" {
			if endpoint.SubscriptionSourceID != "" {
				endpoint.HeartbeatMode = "auto"
			} else {
				endpoint.HeartbeatMode = "required"
			}
		}
	}
	if s.Endpoints == nil {
		s.Endpoints = []clientEndpointRecord{}
	}
	if s.Subscriptions == nil {
		s.Subscriptions = []externalSubscriptionSource{}
	}
	if s.EndpointGroups == nil {
		s.EndpointGroups = []endpointGroup{}
	}
	if s.Redirects == nil {
		s.Redirects = []redirect.Rule{}
	}
	if s.Reverse == nil {
		s.Reverse = make(map[string]clientReverseChannel)
	}
	if s.Forwards == nil {
		s.Forwards = []forward.Rule{}
	}
}

func (s clientInstallState) save(path string) error {
	tree, err := loadOrCreateToml(path)
	if err != nil {
		return err
	}
	s.normalize()
	if err := s.validateHeartbeatModes(); err != nil {
		return err
	}

	if len(s.Endpoints) == 0 {
		tree.DeletePath([]string{"client", "endpoints"})
	} else {
		tree.SetPath([]string{"client", "endpoints"}, s.Endpoints)
	}
	if len(s.Subscriptions) == 0 {
		tree.DeletePath([]string{"client", "subscriptions"})
	} else {
		tree.SetPath([]string{"client", "subscriptions"}, s.Subscriptions)
	}
	if len(s.EndpointGroups) == 0 {
		tree.DeletePath([]string{"client", "endpoint_groups"})
	} else {
		tree.SetPath([]string{"client", "endpoint_groups"}, s.EndpointGroups)
	}
	if len(s.Redirects) == 0 {
		tree.DeletePath([]string{"client", "redirects"})
	} else {
		tree.SetPath([]string{"client", "redirects"}, s.Redirects)
	}
	if len(s.Forwards) == 0 {
		tree.DeletePath([]string{"client", "forwards"})
	} else {
		tree.SetPath([]string{"client", "forwards"}, s.Forwards)
	}
	if len(s.Reverse) == 0 {
		tree.DeletePath([]string{"client", "reverse"})
	} else {
		tree.SetPath([]string{"client", "reverse"}, s.Reverse)
	}
	return writeTomlTree(path, tree)
}

func (s clientInstallState) validateHeartbeatModes() error {
	for _, endpoint := range s.Endpoints {
		switch endpoint.HeartbeatMode {
		case "auto", "required", "disabled":
		default:
			return fmt.Errorf("invalid heartbeat_mode %q for endpoint %q", endpoint.HeartbeatMode, endpoint.Tag)
		}
	}
	return nil
}
