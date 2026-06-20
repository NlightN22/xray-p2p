package ping

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

func verifyPinnedPeerCertificate(rawCerts [][]byte, pin string) error {
	pin = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(pin), ":", ""))
	if pin == "" {
		return errors.New("peer certificate pin is empty")
	}
	if len(rawCerts) == 0 {
		return errors.New("peer certificate is missing")
	}
	sum := sha256.Sum256(rawCerts[0])
	got := hex.EncodeToString(sum[:])
	if got != pin {
		return errors.New("peer certificate pin mismatch")
	}
	return nil
}
