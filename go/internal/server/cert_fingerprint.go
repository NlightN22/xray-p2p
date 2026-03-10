package server

import (
	"crypto/sha256"
	"encoding/hex"
)

func certificateFingerprintSHA256(path string) (string, error) {
	cert, err := loadCertificateFromFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:]), nil
}
