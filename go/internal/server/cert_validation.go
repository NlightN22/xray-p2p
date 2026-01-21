package server

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type pathValidationMode int

const (
	pathValidationStrict pathValidationMode = iota
	pathValidationBasic
)

type certificateValidationError struct {
	err error
}

func (e certificateValidationError) Error() string {
	return e.err.Error()
}

func (e certificateValidationError) Unwrap() error {
	return e.err
}

func isCertificateValidationError(err error) bool {
	var target certificateValidationError
	return errors.As(err, &target)
}

func IsCertificateValidationError(err error) bool {
	return isCertificateValidationError(err)
}

type certificateInputs struct {
	source     string
	certPath   string
	keyPath    string
	selfSigned bool
}

func resolveCertificateInputs(certStore, certPath, keyPath string, relaxed bool) (certificateInputs, error) {
	certPath = strings.TrimSpace(certPath)
	keyPath = strings.TrimSpace(keyPath)
	if certPath == "" && keyPath != "" {
		return certificateInputs{}, certificateValidationError{err: errors.New("xp2p: key file provided without certificate file")}
	}

	source, err := normalizeCertificateSource(certStore, certPath, keyPath)
	if err != nil {
		return certificateInputs{}, certificateValidationError{err: err}
	}

	switch source {
	case CertificateSourceSelfSigned:
		return certificateInputs{source: source, selfSigned: true}, nil
	case CertificateSourcePath:
		if certPath == "" {
			return certificateInputs{}, certificateValidationError{err: errors.New("xp2p: certificate file is required for certificate source path")}
		}
		if keyPath == "" {
			keyPath = certPath
		}
		mode := pathValidationStrict
		if relaxed {
			mode = pathValidationBasic
		}
		if err := validateCertificateFiles(certPath, keyPath, mode); err != nil {
			return certificateInputs{}, certificateValidationError{err: err}
		}
		return certificateInputs{
			source:   source,
			certPath: certPath,
			keyPath:  keyPath,
		}, nil
	case CertificateSourceWinStore:
		if strings.TrimSpace(certStore) == "" {
			return certificateInputs{}, certificateValidationError{err: errors.New("xp2p: certificate store reference is required")}
		}
		return certificateInputs{}, certificateValidationError{err: errors.New("xp2p: certificate source win-store is not implemented")}
	default:
		return certificateInputs{}, certificateValidationError{err: fmt.Errorf("xp2p: unsupported certificate source %q", source)}
	}
}

func validateCertificateFiles(certPath, keyPath string, mode pathValidationMode) error {
	if err := validateAbsolutePath("certificate", certPath, mode); err != nil {
		return err
	}
	if err := validateReadableFile("certificate", certPath); err != nil {
		return err
	}

	if err := validateAbsolutePath("key", keyPath, mode); err != nil {
		return err
	}
	if err := validateReadableFile("key", keyPath); err != nil {
		return err
	}

	warnIfKeyTooPermissive(keyPath)

	if err := validateCertificateKeyMatch(certPath, keyPath); err != nil {
		return err
	}
	return nil
}

func validateAbsolutePath(label, path string, mode pathValidationMode) error {
	if path == "" {
		return fmt.Errorf("xp2p: %s path is empty", label)
	}
	clean := strings.TrimSpace(path)
	if clean == "" {
		return fmt.Errorf("xp2p: %s path is empty", label)
	}
	switch mode {
	case pathValidationBasic:
		if !isBasicAbsolutePath(clean) {
			return fmt.Errorf("xp2p: %s path must be absolute: %s", label, path)
		}
	default:
		if !filepath.IsAbs(clean) {
			return fmt.Errorf("xp2p: %s path must be absolute: %s", label, path)
		}
	}
	return nil
}

func isBasicAbsolutePath(path string) bool {
	if strings.HasPrefix(path, "/") {
		return true
	}
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) {
		return true
	}
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	return false
}

func validateReadableFile(label, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("xp2p: %s file %s: %w", label, path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("xp2p: %s file %s is a directory", label, path)
	}
	handle, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("xp2p: %s file %s: %w", label, path, err)
	}
	if err := handle.Close(); err != nil {
		return fmt.Errorf("xp2p: %s file %s: %w", label, path, err)
	}
	return nil
}

func validateCertificateKeyMatch(certPath, keyPath string) error {
	cert, err := loadCertificateFromFile(certPath)
	if err != nil {
		return err
	}
	key, err := loadPrivateKeyFromFile(keyPath)
	if err != nil {
		return err
	}
	keyPub, err := publicKeyFromPrivateKey(key)
	if err != nil {
		return err
	}
	matches, err := publicKeysMatch(cert.PublicKey, keyPub)
	if err != nil {
		return err
	}
	if !matches {
		return fmt.Errorf("xp2p: certificate and key do not match")
	}
	return nil
}

func loadCertificateFromFile(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("xp2p: read certificate %s: %w", path, err)
	}
	var block *pem.Block
	for len(data) > 0 {
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			break
		}
	}
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("xp2p: decode certificate %s: invalid PEM data", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("xp2p: parse certificate %s: %w", path, err)
	}
	return cert, nil
}

func loadPrivateKeyFromFile(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("xp2p: read key %s: %w", path, err)
	}
	for len(data) > 0 {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		switch block.Type {
		case "RSA PRIVATE KEY":
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("xp2p: parse key %s: %w", path, err)
			}
			return key, nil
		case "EC PRIVATE KEY":
			key, err := x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("xp2p: parse key %s: %w", path, err)
			}
			return key, nil
		case "PRIVATE KEY":
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("xp2p: parse key %s: %w", path, err)
			}
			return key, nil
		}
	}
	return nil, fmt.Errorf("xp2p: decode key %s: invalid PEM data", path)
}

func publicKeyFromPrivateKey(key any) (any, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey, nil
	case *ecdsa.PrivateKey:
		return &k.PublicKey, nil
	case ed25519.PrivateKey:
		return k.Public().(ed25519.PublicKey), nil
	case crypto.Signer:
		return k.Public(), nil
	default:
		return nil, fmt.Errorf("xp2p: unsupported private key type %T", key)
	}
}

func publicKeysMatch(certKey, key any) (bool, error) {
	certBytes, err := x509.MarshalPKIXPublicKey(certKey)
	if err != nil {
		return false, fmt.Errorf("xp2p: encode certificate public key: %w", err)
	}
	keyBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return false, fmt.Errorf("xp2p: encode key public key: %w", err)
	}
	return bytes.Equal(certBytes, keyBytes), nil
}
