package server

import (
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func defaultTLSDir() string {
	return filepath.Join(config.ConfigRoot(), "tls", "server")
}

func defaultCertPath() string {
	return filepath.Join(defaultTLSDir(), "cert.pem")
}

func defaultKeyPath() string {
	return filepath.Join(defaultTLSDir(), "key.pem")
}

func defaultTLSConfigured() bool {
	if _, err := os.Stat(defaultCertPath()); err != nil {
		return false
	}
	if _, err := os.Stat(defaultKeyPath()); err != nil {
		return false
	}
	return true
}
