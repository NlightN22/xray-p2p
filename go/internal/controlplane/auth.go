package controlplane

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxNonceLen = 128

var (
	ErrAuthRequired = errors.New("control auth required")
	ErrAuthInvalid  = errors.New("control auth invalid")
)

func Sign(secret, method, path, rawQuery string, body []byte, timestamp time.Time, nonce string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonicalString(method, path, rawQuery, bodyHash(body), timestamp.Unix(), nonce)))
	return hex.EncodeToString(mac.Sum(nil))
}

// RotationProof deliberately signs only a server-issued nonce. Rotation is independent
// from the general request-header authentication protocol.
func RotationProof(secret, nonce string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(nonce))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyRequest(r *http.Request, body []byte, users []AuthUser, now time.Time, window time.Duration) error {
	if len(users) == 0 {
		return nil
	}
	user := strings.TrimSpace(r.Header.Get(HeaderUser))
	tsRaw := strings.TrimSpace(r.Header.Get(HeaderTimestamp))
	nonce := strings.TrimSpace(r.Header.Get(HeaderNonce))
	sig := strings.TrimSpace(r.Header.Get(HeaderSignature))
	if user == "" || tsRaw == "" || nonce == "" || sig == "" {
		return ErrAuthRequired
	}
	if len(nonce) > maxNonceLen || strings.ContainsAny(nonce, " \t\r\n") {
		return ErrAuthInvalid
	}
	ts, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil {
		return ErrAuthInvalid
	}
	if window <= 0 {
		window = 120 * time.Second
	}
	requestAt := time.Unix(ts, 0).UTC()
	delta := now.UTC().Sub(requestAt)
	if delta < -window || delta > window {
		return ErrAuthInvalid
	}
	var secrets []string
	for _, candidate := range users {
		if strings.EqualFold(strings.TrimSpace(candidate.Label), user) {
			if secret := strings.TrimSpace(candidate.Credential); secret != "" {
				secrets = append(secrets, secret)
			}
		}
	}
	if len(secrets) == 0 {
		return ErrAuthInvalid
	}
	got, err := hex.DecodeString(sig)
	if err != nil {
		return ErrAuthInvalid
	}
	for _, secret := range secrets {
		expected := Sign(secret, r.Method, r.URL.Path, r.URL.RawQuery, body, requestAt, nonce)
		want, _ := hex.DecodeString(expected)
		if hmac.Equal(got, want) {
			return nil
		}
	}
	return ErrAuthInvalid
}

func ApplyHeaders(req *http.Request, user, secret, nonce string, body []byte, now time.Time) error {
	if req == nil {
		return errors.New("request is nil")
	}
	user = strings.TrimSpace(user)
	secret = strings.TrimSpace(secret)
	nonce = strings.TrimSpace(nonce)
	if user == "" || secret == "" || nonce == "" {
		return errors.New("control auth user, secret, and nonce are required")
	}
	req.Header.Set(HeaderUser, user)
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(now.UTC().Unix(), 10))
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderSignature, Sign(secret, req.Method, req.URL.Path, req.URL.RawQuery, body, now, nonce))
	return nil
}

func canonicalString(method, path, rawQuery, hash string, ts int64, nonce string) string {
	query := ""
	if rawQuery != "" {
		values, err := url.ParseQuery(rawQuery)
		if err == nil {
			query = values.Encode()
		} else {
			query = rawQuery
		}
	}
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%d\n%s",
		strings.ToUpper(method), path, query, hash, ts, nonce)
}

func bodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
