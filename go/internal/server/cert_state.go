//go:build windows || linux

package server

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func CertificateStateFromConfig(opts CertificateStateOptions) (CertificateState, error) {
	_ = opts.InstallDir
	_ = opts.ConfigDir
	_ = opts.Pending

	cfg, err := config.Load(config.Options{
		Path:         pendingConfigPath(),
		AllowInvalid: true,
	})
	if err != nil {
		return CertificateState{}, err
	}

	certPath := strings.TrimSpace(cfg.Server.CertificateFile)
	keyPath := strings.TrimSpace(cfg.Server.KeyFile)
	if certPath == "" && keyPath == "" && defaultTLSConfigured() {
		certPath = defaultCertPath()
		keyPath = defaultKeyPath()
	}
	if certPath == "" || keyPath == "" {
		certPath = ""
		keyPath = ""
	}

	state := CertificateState{
		CertPath: certPath,
		KeyPath:  keyPath,
		Status:   CertificateStatusMissing,
	}

	if certPath == "" || keyPath == "" {
		state.Issues = append(state.Issues, "xp2p: certificate paths are not configured")
		return state, nil
	}

	cert, status, issue := loadCertificateDetails(state.CertPath)
	state.Status = status
	if issue != nil {
		state.Issues = append(state.Issues, issue.Error())
	}

	if cert != nil {
		state.Subject = cert.Subject.String()
		state.DNSNames = cert.DNSNames
		for _, ip := range cert.IPAddresses {
			state.IPAddresses = append(state.IPAddresses, ip.String())
		}
		state.NotBefore = cert.NotBefore
		state.NotAfter = cert.NotAfter
		state.RemainingDays = daysUntil(cert.NotAfter)
		state.SelfSigned = isCertificateSelfSigned(cert)
	}

	if keyIssue := probeKeyFile(state.KeyPath); keyIssue != nil {
		state.Issues = append(state.Issues, keyIssue.Error())
		if state.Status == CertificateStatusOK {
			state.Status = CertificateStatusMissing
		}
	}

	return state, nil
}

func loadCertificateDetails(certPath string) (*x509.Certificate, CertificateStatus, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, CertificateStatusMissing, fmt.Errorf("xp2p: certificate %s not found", certPath)
		}
		return nil, CertificateStatusParseError, fmt.Errorf("xp2p: read certificate %s: %w", certPath, err)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, CertificateStatusParseError, fmt.Errorf("xp2p: decode certificate %s: invalid PEM data", certPath)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, CertificateStatusParseError, fmt.Errorf("xp2p: parse certificate %s: %w", certPath, err)
	}

	now := time.Now()
	switch {
	case now.Before(cert.NotBefore):
		return cert, CertificateStatusNotYetValid, nil
	case now.After(cert.NotAfter):
		return cert, CertificateStatusExpired, nil
	default:
		return cert, CertificateStatusOK, nil
	}
}

func probeKeyFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: key %s not found", path)
		}
		return fmt.Errorf("xp2p: key %s: %w", path, err)
	}
	return nil
}

func daysUntil(ts time.Time) int {
	return int(math.Floor(time.Until(ts).Hours() / 24))
}

func isCertificateSelfSigned(cert *x509.Certificate) bool {
	if cert == nil {
		return false
	}
	if cert.Subject.String() != cert.Issuer.String() {
		return false
	}
	return cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) == nil
}
