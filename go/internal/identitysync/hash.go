package identitysync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func HashJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return HashBytes(data), nil
}
