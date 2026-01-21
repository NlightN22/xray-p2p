package server

import (
	"fmt"
	"strings"
)

const (
	CertificateSourceSelfSigned = "self-signed"
	CertificateSourcePath       = "path"
	CertificateSourceWinStore   = "win-store"
)

func normalizeCertificateSource(storeRef, certPath, keyPath string) (string, error) {
	hasStore := strings.TrimSpace(storeRef) != ""
	hasCert := strings.TrimSpace(certPath) != "" || strings.TrimSpace(keyPath) != ""
	if hasStore && hasCert {
		return "", fmt.Errorf("xp2p: certificate store cannot be combined with --cert/--key")
	}
	if hasStore {
		return CertificateSourceWinStore, nil
	}
	if hasCert {
		return CertificateSourcePath, nil
	}
	return CertificateSourceSelfSigned, nil
}
