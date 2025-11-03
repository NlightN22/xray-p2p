package server

import "errors"

// ErrCertificateConfigured indicates that a TLS certificate is already present and confirmation is required.
var ErrCertificateConfigured = errors.New("xp2p: tls certificate already configured")
