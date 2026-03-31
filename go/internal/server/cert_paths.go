package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
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
	return resolveCertificatePathWithPending(configDir, path, false)
}

func resolveCertificatePathWithPending(configDir, path string, pending bool) string {
	resolved := filepath.FromSlash(strings.TrimSpace(path))
	if filepath.IsAbs(resolved) {
		if !pending {
			return resolved
		}
		if _, err := os.Stat(resolved); err == nil {
			return resolved
		}
		if liveDir, ok := liveDirFromPending(configDir); ok {
			if rel, err := filepath.Rel(liveDir, resolved); err == nil {
				candidate := filepath.Join(configDir, rel)
				if _, err := os.Stat(candidate); err == nil {
					return candidate
				}
			}
		}
		return resolved
	}

	candidate := filepath.Join(configDir, resolved)
	if !pending {
		return candidate
	}
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	if liveDir, ok := liveDirFromPending(configDir); ok {
		liveCandidate := filepath.Join(liveDir, resolved)
		if _, err := os.Stat(liveCandidate); err == nil {
			return liveCandidate
		}
	}
	return candidate
}

func isPendingConfigDir(configDir string) bool {
	_, ok := liveDirFromPending(configDir)
	return ok
}

func liveDirFromPending(configDir string) (string, bool) {
	clean := filepath.Clean(configDir)
	pendingSuffix := filepath.Join(layout.ApplyDirName, layout.PendingDirName)
	if !strings.HasSuffix(clean, pendingSuffix) {
		return "", false
	}
	base := strings.TrimSuffix(clean, pendingSuffix)
	base = strings.TrimSuffix(base, string(filepath.Separator))
	if strings.TrimSpace(base) == "" {
		return "", false
	}
	return base, true
}
