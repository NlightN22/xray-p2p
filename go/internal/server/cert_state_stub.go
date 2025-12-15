//go:build !windows && !linux

package server

func CertificateStateFromConfig(_ CertificateStateOptions) (CertificateState, error) {
	return CertificateState{}, ErrUnsupported
}
