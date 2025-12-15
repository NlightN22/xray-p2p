package server

import (
	"errors"
	"path/filepath"
	"strings"
)

func certificatePathsFromStream(stream map[string]any) (string, string, error) {
	tlsSettings, ok := stream["tlsSettings"].(map[string]any)
	if !ok {
		return "", "", errors.New("xp2p: tlsSettings missing in trojan stream settings")
	}

	rawCerts, ok := tlsSettings["certificates"].([]any)
	if !ok || len(rawCerts) == 0 {
		return "", "", errors.New("xp2p: no TLS certificates configured")
	}

	entry, ok := rawCerts[0].(map[string]any)
	if !ok {
		return "", "", errors.New("xp2p: tls certificate entry invalid")
	}

	rawCertPath, _ := entry["certificateFile"].(string)
	certPath := strings.TrimSpace(rawCertPath)
	if certPath == "" {
		return "", "", errors.New("xp2p: certificateFile missing in TLS configuration")
	}

	rawKeyPath, _ := entry["keyFile"].(string)
	keyPath := strings.TrimSpace(rawKeyPath)
	if keyPath == "" {
		keyPath = certPath
	}

	return certPath, keyPath, nil
}

func resolveCertificatePath(configDir, path string) string {
	resolved := filepath.FromSlash(strings.TrimSpace(path))
	if filepath.IsAbs(resolved) {
		return resolved
	}
	return filepath.Join(configDir, resolved)
}
