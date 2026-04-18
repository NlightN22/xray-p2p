package client

import (
	"fmt"
	"strings"
)

func (s *clientInstallState) removeEndpoint(target string) (clientEndpointRecord, bool) {
	if len(s.Endpoints) == 0 {
		return clientEndpointRecord{}, false
	}
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return clientEndpointRecord{}, false
	}
	lower := strings.ToLower(trimmed)
	for idx, ep := range s.Endpoints {
		if strings.EqualFold(ep.Hostname, trimmed) || strings.ToLower(ep.Tag) == lower {
			removed := ep
			s.Endpoints = append(s.Endpoints[:idx], s.Endpoints[idx+1:]...)
			return removed, true
		}
	}
	return clientEndpointRecord{}, false
}

func (s *clientInstallState) upsert(record clientEndpointRecord, force bool) error {
	for idx, existing := range s.Endpoints {
		sameHost := strings.EqualFold(existing.Hostname, record.Hostname)
		samePort := existing.Port == record.Port
		if sameHost && samePort {
			if !force {
				return fmt.Errorf("endpoint %s:%d already exists (use --force to update)", record.Hostname, record.Port)
			}
			s.Endpoints[idx] = record
			return nil
		}
		if sameHost {
			if !force {
				return fmt.Errorf("endpoint %s already exists (use --force to update)", record.Hostname)
			}
			s.Endpoints[idx] = record
			return nil
		}
		if strings.EqualFold(existing.Tag, record.Tag) {
			return fmt.Errorf("outbound tag %s is already assigned to %s", record.Tag, existing.Hostname)
		}
	}
	s.Endpoints = append(s.Endpoints, record)
	return nil
}

func (s *clientInstallState) applyAllowInsecure(value bool) {
	for idx := range s.Endpoints {
		s.Endpoints[idx].AllowInsecure = value
	}
}
