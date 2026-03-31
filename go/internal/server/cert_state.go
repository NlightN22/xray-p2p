//go:build windows || linux

package server

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

func CertificateStateFromConfig(opts CertificateStateOptions) (CertificateState, error) {
	configDir, err := resolveCertificateConfigDir(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return CertificateState{}, err
	}
	if opts.Pending {
		configDir = pendingConfigDir(configDir)
	}
	if err := ensureConfigExists(configDir); err != nil {
		return CertificateState{}, err
	}

	trojan, err := loadTrojanState(configDir)
	if err != nil {
		return CertificateState{}, err
	}

	state := CertificateState{
		CertPath: filepath.Join(configDir, "cert.pem"),
		KeyPath:  filepath.Join(configDir, "key.pem"),
		Status:   CertificateStatusMissing,
	}

	certRel, keyRel, err := certificatePathsFromStream(trojan.stream)
	if err != nil {
		state.Issues = append(state.Issues, err.Error())
		return state, nil
	}

	state.CertPath = resolveCertificatePathWithPending(configDir, certRel, opts.Pending)
	state.KeyPath = resolveCertificatePathWithPending(configDir, keyRel, opts.Pending)

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
