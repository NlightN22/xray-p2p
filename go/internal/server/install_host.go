package server

import (
	"fmt"
	"net"
	"strings"
)

func validateCertificateHost(host string) error {
	if net.ParseIP(host) != nil {
		return nil
	}

	if len(host) > 253 {
		return fmt.Errorf("invalid host %q", host)
	}

	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return fmt.Errorf("invalid host")
	}

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return fmt.Errorf("invalid host label %q", label)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("invalid host label %q", label)
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' {
				continue
			}
			return fmt.Errorf("invalid host label %q", label)
		}
	}
	return nil
}
