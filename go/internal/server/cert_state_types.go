package server

import "time"

type CertificateStatus string

const (
	CertificateStatusOK          CertificateStatus = "ok"
	CertificateStatusExpired     CertificateStatus = "expired"
	CertificateStatusNotYetValid CertificateStatus = "not yet valid"
	CertificateStatusMissing     CertificateStatus = "missing"
	CertificateStatusParseError  CertificateStatus = "parse error"
)

type CertificateStateOptions struct {
	InstallDir string
	ConfigDir  string
}

type CertificateState struct {
	CertPath      string
	KeyPath       string
	Subject       string
	DNSNames      []string
	IPAddresses   []string
	NotBefore     time.Time
	NotAfter      time.Time
	Status        CertificateStatus
	RemainingDays int
	Issues        []string
}
